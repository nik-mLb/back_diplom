package recommendation

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/transport/middleware/logctx"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// bayesConfidenceM — порог доверия (m) в байесовском взвешенном рейтинге:
// score = v/(v+m)*R + m/(v+m)*C, где v = число отзывов товара, R — его средний
// рейтинг, C — средний рейтинг по каталогу. Чем больше m, тем сильнее товар с
// малым числом отзывов «притягивается» к среднему по каталогу (не даёт одному
// отзыву 5★ обойти товар с сотнями отзывов). Подбирается на реальных данных.
const bayesConfidenceM = 20

const (
	queryGetSubcategoryByProduct = `
		SELECT subcategory_id
		FROM bazaar.product_subcategory
		WHERE product_id = $1`

	// Топ товаров подкатегории по байесовскому взвешенному рейтингу (контентный слой).
	queryTopProductIDsBySubcategory = `
		WITH stats AS (
			SELECT COALESCE(AVG(rating), 0) AS c
			FROM bazaar.product WHERE reviews_count > 0
		)
		SELECT ps.product_id
		FROM bazaar.product_subcategory ps
		JOIN bazaar.product p ON p.id = ps.product_id
		CROSS JOIN stats
		WHERE ps.subcategory_id = $1
		  AND p.status = 'approved'
		  AND p.quantity > 0
		ORDER BY (p.reviews_count::float / (p.reviews_count + $2)) * p.rating
		       + ($2::float / (p.reviews_count + $2)) * stats.c DESC
		LIMIT $3`

	// «С этим товаром покупают»: товары из тех же заказов, что и заданный
	// (self-join по order_item), ранжир по частоте совместных покупок.
	queryCoPurchasedProductIDs = `
		SELECT oi2.product_id
		FROM bazaar.order_item oi1
		JOIN bazaar.order_item oi2
		     ON oi1.order_id = oi2.order_id AND oi2.product_id <> oi1.product_id
		JOIN bazaar.product p ON p.id = oi2.product_id
		WHERE oi1.product_id = $1
		  AND p.status = 'approved'
		  AND p.quantity > 0
		GROUP BY oi2.product_id
		ORDER BY COUNT(*) DESC
		LIMIT $2`

	// Любимые подкатегории пользователя по частоте покупок (для персонализации).
	queryPreferredSubcategoryIDs = `
		SELECT ps.subcategory_id
		FROM bazaar."order" o
		JOIN bazaar.order_item oi ON oi.order_id = o.id
		JOIN bazaar.product_subcategory ps ON ps.product_id = oi.product_id
		WHERE o.user_id = $1
		GROUP BY ps.subcategory_id
		ORDER BY COUNT(*) DESC
		LIMIT $2`

	// Товары, которые пользователь уже покупал (чтобы исключить из выдачи).
	queryPurchasedProductIDs = `
		SELECT DISTINCT oi.product_id
		FROM bazaar."order" o
		JOIN bazaar.order_item oi ON oi.order_id = o.id
		WHERE o.user_id = $1`

	// Глобально популярные товары по байесовскому рейтингу (cold-start).
	queryPopularProductIDs = `
		WITH stats AS (
			SELECT COALESCE(AVG(rating), 0) AS c
			FROM bazaar.product WHERE reviews_count > 0
		)
		SELECT p.id
		FROM bazaar.product p
		CROSS JOIN stats
		WHERE p.status = 'approved'
		  AND p.quantity > 0
		ORDER BY (p.reviews_count::float / (p.reviews_count + $1)) * p.rating
		       + ($1::float / (p.reviews_count + $1)) * stats.c DESC
		LIMIT $2`

	// Комплементарные товары: «те, кто покупал ЭТИ товары, брали ещё…». Берём
	// заказы, где встречался любой из переданных товаров, и агрегируем остальные
	// позиции по частоте. Сами переданные товары исключаем ($1 — их массив).
	queryComplementaryProductIDs = `
		SELECT oi2.product_id
		FROM bazaar.order_item oi1
		JOIN bazaar.order_item oi2
		     ON oi1.order_id = oi2.order_id AND oi2.product_id <> oi1.product_id
		JOIN bazaar.product p ON p.id = oi2.product_id
		WHERE oi1.product_id = ANY($1::uuid[])
		  AND oi2.product_id <> ALL($1::uuid[])
		  AND p.status = 'approved'
		  AND p.quantity > 0
		GROUP BY oi2.product_id
		ORDER BY COUNT(*) DESC
		LIMIT $2`
)

//go:generate mockgen -source=recommendation.go -destination=../mocks/recommendation_repository_mock.go -package=mocks IRecommendationRepository
type IRecommendationRepository interface {
	GetCategoryIDsByProductID(context.Context, uuid.UUID) ([]uuid.UUID, error)
	GetProductIDsBySubcategoryID(ctx context.Context, subcategoryID uuid.UUID, count int) ([]uuid.UUID, error)
	GetCoPurchasedProductIDs(ctx context.Context, productID uuid.UUID, limit int) ([]uuid.UUID, error)
	GetPreferredSubcategoryIDs(ctx context.Context, userID uuid.UUID, limit int) ([]uuid.UUID, error)
	GetPurchasedProductIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	GetComplementaryProductIDs(ctx context.Context, purchasedIDs []uuid.UUID, limit int) ([]uuid.UUID, error)
	GetPopularProductIDs(ctx context.Context, limit int) ([]uuid.UUID, error)
}

type RecommendationRepository struct {
	db *sql.DB
}

func NewRecommendationRepository(db *sql.DB) *RecommendationRepository {
	return &RecommendationRepository{
		db: db,
	}
}

// queryUUIDList выполняет запрос, возвращающий один UUID-столбец, и собирает его в срез.
func (r *RecommendationRepository) queryUUIDList(ctx context.Context, op, query string, args ...any) ([]uuid.UUID, error) {
	logger := logctx.GetLogger(ctx).WithField("op", op)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		logger.WithError(err).Error("execute query")
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			logger.WithError(err).Error("scan row")
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		logger.WithError(err).Error("rows iteration error")
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return ids, nil
}

func (r *RecommendationRepository) GetCategoryIDsByProductID(ctx context.Context, productID uuid.UUID) ([]uuid.UUID, error) {
	return r.queryUUIDList(ctx, "RecommendationRepository.GetCategoryIDsByProductID",
		queryGetSubcategoryByProduct, productID)
}

func (r *RecommendationRepository) GetProductIDsBySubcategoryID(ctx context.Context, subcategoryID uuid.UUID, count int) ([]uuid.UUID, error) {
	return r.queryUUIDList(ctx, "RecommendationRepository.GetProductIDsBySubcategoryID",
		queryTopProductIDsBySubcategory, subcategoryID, bayesConfidenceM, count)
}

func (r *RecommendationRepository) GetCoPurchasedProductIDs(ctx context.Context, productID uuid.UUID, limit int) ([]uuid.UUID, error) {
	return r.queryUUIDList(ctx, "RecommendationRepository.GetCoPurchasedProductIDs",
		queryCoPurchasedProductIDs, productID, limit)
}

func (r *RecommendationRepository) GetPreferredSubcategoryIDs(ctx context.Context, userID uuid.UUID, limit int) ([]uuid.UUID, error) {
	return r.queryUUIDList(ctx, "RecommendationRepository.GetPreferredSubcategoryIDs",
		queryPreferredSubcategoryIDs, userID, limit)
}

func (r *RecommendationRepository) GetPurchasedProductIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	return r.queryUUIDList(ctx, "RecommendationRepository.GetPurchasedProductIDs",
		queryPurchasedProductIDs, userID)
}

func (r *RecommendationRepository) GetPopularProductIDs(ctx context.Context, limit int) ([]uuid.UUID, error) {
	return r.queryUUIDList(ctx, "RecommendationRepository.GetPopularProductIDs",
		queryPopularProductIDs, bayesConfidenceM, limit)
}

func (r *RecommendationRepository) GetComplementaryProductIDs(ctx context.Context, purchasedIDs []uuid.UUID, limit int) ([]uuid.UUID, error) {
	if len(purchasedIDs) == 0 {
		return nil, nil
	}
	// lib/pq не умеет []uuid.UUID напрямую — передаём массив строк с приведением ::uuid[].
	strs := make([]string, len(purchasedIDs))
	for i, id := range purchasedIDs {
		strs[i] = id.String()
	}
	return r.queryUUIDList(ctx, "RecommendationRepository.GetComplementaryProductIDs",
		queryComplementaryProductIDs, pq.Array(strs), limit)
}
