package recommendation

import (
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/models/domains"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/models/errs"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/transport/dto"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/transport/middleware/logctx"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/transport/utils/response"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/usecase/recommendation"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"net/http"
)

// PersonalRecommendationsResponse — ответ рекомендаций для главной страницы.
// Personalized=true, если выдача построена по истории пользователя; false —
// если это глобально популярное (холодный старт / гость). Фронт по этому флагу
// выбирает заголовок блока («Рекомендуем вам» / «Популярное»).
type PersonalRecommendationsResponse struct {
	Total        int                `json:"total"`
	Personalized bool               `json:"personalized"`
	Products     []dto.BriefProduct `json:"products"`
}

type RecommendationServise struct {
	r recommendation.IRecommendationUsecase
}

func NewRecommendationService(r recommendation.IRecommendationUsecase) *RecommendationServise {
	return &RecommendationServise{
		r: r,
	}
}

func (h *RecommendationServise) GetRecommendations(w http.ResponseWriter, r *http.Request) {
	const op = "ProductService.GetAllProducts"
	logger := logctx.GetLogger(r.Context()).WithField("op", op)

	vars := mux.Vars(r)
	productID := vars["id"]

	parseProductID, err := uuid.Parse(productID)
	if err != nil {
		logger.WithError(err).WithField("productID", parseProductID).Error("parse productID")
		response.HandleDomainError(r.Context(), w, errs.ErrParseRequestData, op)
		return
	}

	recommendations, err := h.r.GetRecommendations(r.Context(), parseProductID)
	if err != nil {
		logger.WithError(err).WithField("productID", parseProductID).Error("get recommendations")
		response.HandleDomainError(r.Context(), w, err, op)
		return
	}

	recommendationsResp := dto.ConvertToProductsResponse(recommendations)

	response.SendJSONResponse(r.Context(), w, http.StatusOK, recommendationsResp)
}

// GetPersonalRecommendations — рекомендации для главной страницы. userID берётся
// из контекста (OptionalJWTMiddleware): если токена нет — userID нулевой, и usecase
// вернёт глобально популярное. Эндпоинт работает и для гостя, и для залогиненного.
func (h *RecommendationServise) GetPersonalRecommendations(w http.ResponseWriter, r *http.Request) {
	const op = "RecommendationService.GetPersonalRecommendations"
	logger := logctx.GetLogger(r.Context()).WithField("op", op)

	userID := uuid.Nil
	if v, ok := r.Context().Value(domains.UserIDKey{}).(string); ok && v != "" {
		if parsed, err := uuid.Parse(v); err == nil {
			userID = parsed
		}
	}

	products, personalized, err := h.r.GetPersonalRecommendations(r.Context(), userID)
	if err != nil {
		logger.WithError(err).WithField("userID", userID).Error("get personal recommendations")
		response.HandleDomainError(r.Context(), w, err, op)
		return
	}

	resp := dto.ConvertToProductsResponse(products)
	response.SendJSONResponse(r.Context(), w, http.StatusOK, PersonalRecommendationsResponse{
		Total:        resp.Total,
		Personalized: personalized,
		Products:     resp.Products,
	})
}
