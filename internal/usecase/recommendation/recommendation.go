package recommendation

import (
	"context"
	"fmt"
	recommendationRepo "github.com/go-park-mail-ru/2025_1_ChillGuys/internal/infrastructure/repository/postgres/recommendation"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/models"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/transport/middleware/logctx"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/transport/product"
	"github.com/google/uuid"
)

const (
	// itemToItemLimit — сколько товаров показываем в блоке на карточке товара.
	itemToItemLimit = 10
	// personalLimit — сколько товаров показываем в блоке на главной.
	personalLimit = 20
	// preferredSubcatLimit — сколько любимых подкатегорий берём в профиль юзера.
	preferredSubcatLimit = 5
)

//go:generate mockgen -source=recommendation.go -destination=../mocks/recommendation_usecase_mock.go -package=mocks IRecommendationUsecase
type IRecommendationUsecase interface {
	// GetRecommendations — рекомендации к товару (item-to-item): co-purchase + контент.
	GetRecommendations(ctx context.Context, productID uuid.UUID) ([]*models.Product, error)
	// GetPersonalRecommendations — рекомендации для главной. Второй возвращаемый
	// параметр personalized=true, если выдача построена по истории пользователя,
	// и false, если это глобально популярное (cold-start / аноним).
	GetPersonalRecommendations(ctx context.Context, userID uuid.UUID) ([]*models.Product, bool, error)
}

type RecommendationUsecase struct {
	pu product.IProductUsecase
	rr recommendationRepo.IRecommendationRepository
}

func NewRecommendationUsecase(
	productUscase product.IProductUsecase,
	recommendationRepo recommendationRepo.IRecommendationRepository,
) *RecommendationUsecase {
	return &RecommendationUsecase{
		rr: recommendationRepo,
		pu: productUscase,
	}
}

// GetRecommendations строит item-to-item выдачу для карточки товара:
// сначала товары, которые реально покупают вместе с данным (co-purchase),
// затем добор похожими по подкатегории (ранжир по байесовскому рейтингу).
func (u *RecommendationUsecase) GetRecommendations(ctx context.Context, productID uuid.UUID) ([]*models.Product, error) {
	const op = "RecommendationUsecase.GetRecommendations"
	logger := logctx.GetLogger(ctx).WithField("op", op)

	seen := map[uuid.UUID]bool{productID: true} // сам товар исключаем
	ordered := make([]uuid.UUID, 0, itemToItemLimit)

	appendIDs := func(ids []uuid.UUID) {
		for _, id := range ids {
			if len(ordered) >= itemToItemLimit {
				return
			}
			if !seen[id] {
				seen[id] = true
				ordered = append(ordered, id)
			}
		}
	}

	// Слой A — поведенческий: «с этим товаром покупают».
	coPurchased, err := u.rr.GetCoPurchasedProductIDs(ctx, productID, itemToItemLimit)
	if err != nil {
		logger.WithError(err).Error("get co-purchased product ids")
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	appendIDs(coPurchased)

	// Слой B — контентный добор: похожие товары из тех же подкатегорий.
	if len(ordered) < itemToItemLimit {
		subcatIDs, err := u.rr.GetCategoryIDsByProductID(ctx, productID)
		if err != nil {
			logger.WithError(err).Error("get subcategory ids")
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		for _, subcatID := range subcatIDs {
			if len(ordered) >= itemToItemLimit {
				break
			}
			ids, err := u.rr.GetProductIDsBySubcategoryID(ctx, subcatID, itemToItemLimit)
			if err != nil {
				logger.WithError(err).WithField("subcategory_id", subcatID).
					Warn("failed to get products by subcategory")
				continue
			}
			appendIDs(ids)
		}
	}

	if len(ordered) == 0 {
		return nil, nil
	}

	products, err := u.pu.GetProductsByIDs(ctx, ordered)
	if err != nil {
		logger.WithError(err).Error("get products by ids")
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return products, nil
}

// GetPersonalRecommendations строит выдачу для главной страницы.
// Лесенка: есть покупки → комплементарные товары («с этим покупают» от истории
// пользователя) + добор похожими из любимых подкатегорий; нет истории / аноним
// → глобально популярное.
func (u *RecommendationUsecase) GetPersonalRecommendations(ctx context.Context, userID uuid.UUID) ([]*models.Product, bool, error) {
	const op = "RecommendationUsecase.GetPersonalRecommendations"
	logger := logctx.GetLogger(ctx).WithField("op", op)

	// Аноним — сразу популярное.
	if userID == uuid.Nil {
		return u.popular(ctx, false)
	}

	purchased, err := u.rr.GetPurchasedProductIDs(ctx, userID)
	if err != nil {
		logger.WithError(err).Error("get purchased product ids")
		return nil, false, fmt.Errorf("%s: %w", op, err)
	}
	// Нет истории покупок → cold-start → популярное.
	if len(purchased) == 0 {
		return u.popular(ctx, false)
	}

	excluded := make(map[uuid.UUID]bool, len(purchased))
	for _, id := range purchased {
		excluded[id] = true
	}

	seen := make(map[uuid.UUID]bool)
	ordered := make([]uuid.UUID, 0, personalLimit)
	add := func(id uuid.UUID) {
		if excluded[id] || seen[id] {
			return
		}
		seen[id] = true
		ordered = append(ordered, id)
	}

	// Слой 1 — комплементарные: «те, кто покупал ваши товары, брали ещё…».
	complementary, err := u.rr.GetComplementaryProductIDs(ctx, purchased, personalLimit)
	if err != nil {
		logger.WithError(err).Error("get complementary product ids")
		return nil, false, fmt.Errorf("%s: %w", op, err)
	}
	for _, id := range complementary {
		if len(ordered) >= personalLimit {
			break
		}
		add(id)
	}

	// Слой 2 — добор похожими из любимых подкатегорий (round-robin), если
	// комплементарных не хватило. Ошибки здесь не фатальны.
	if len(ordered) < personalLimit {
		preferred, err := u.rr.GetPreferredSubcategoryIDs(ctx, userID, preferredSubcatLimit)
		if err != nil {
			logger.WithError(err).Warn("get preferred subcategory ids")
		} else {
			lists := make([][]uuid.UUID, 0, len(preferred))
			for _, subcatID := range preferred {
				ids, err := u.rr.GetProductIDsBySubcategoryID(ctx, subcatID, personalLimit)
				if err != nil {
					logger.WithError(err).WithField("subcategory_id", subcatID).
						Warn("failed to get products by subcategory")
					continue
				}
				lists = append(lists, ids)
			}
			for round := 0; len(ordered) < personalLimit; round++ {
				progressed := false
				for _, list := range lists {
					if round >= len(list) {
						continue
					}
					progressed = true
					if len(ordered) >= personalLimit {
						break
					}
					add(list[round])
				}
				if !progressed {
					break
				}
			}
		}
	}

	// Ничего не набралось → откат на популярное.
	if len(ordered) == 0 {
		return u.popular(ctx, false)
	}

	products, err := u.pu.GetProductsByIDs(ctx, ordered)
	if err != nil {
		logger.WithError(err).Error("get products by ids")
		return nil, false, fmt.Errorf("%s: %w", op, err)
	}

	return products, true, nil
}

// popular возвращает глобально популярные товары (байесовский рейтинг).
// personalized прокидывается как есть — вызывающая сторона решает флаг заранее.
func (u *RecommendationUsecase) popular(ctx context.Context, personalized bool) ([]*models.Product, bool, error) {
	const op = "RecommendationUsecase.popular"
	logger := logctx.GetLogger(ctx).WithField("op", op)

	ids, err := u.rr.GetPopularProductIDs(ctx, personalLimit)
	if err != nil {
		logger.WithError(err).Error("get popular product ids")
		return nil, false, fmt.Errorf("%s: %w", op, err)
	}
	if len(ids) == 0 {
		return nil, personalized, nil
	}

	products, err := u.pu.GetProductsByIDs(ctx, ids)
	if err != nil {
		logger.WithError(err).Error("get products by ids")
		return nil, false, fmt.Errorf("%s: %w", op, err)
	}

	return products, personalized, nil
}
