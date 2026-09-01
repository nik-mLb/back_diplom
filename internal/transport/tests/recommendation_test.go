package tests

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/models"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/models/domains"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/transport/recommendation"
	mockRecommendation "github.com/go-park-mail-ru/2025_1_ChillGuys/internal/usecase/mocks"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetRecommendations_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUsecase := mockRecommendation.NewMockIRecommendationUsecase(ctrl)
	handler := recommendation.NewRecommendationService(mockUsecase)

	productID := uuid.New()
	expectedProducts := []*models.Product{
		{
			ID:              uuid.New(),
			Name:            "Test Product",
			PreviewImageURL: "/img/test.png",
			Price:           100.0,
			PriceDiscount:   90.0,
			Quantity:        10,
			Rating:          4.5,
			ReviewsCount:    5,
		},
	}

	mockUsecase.EXPECT().
		GetRecommendations(gomock.Any(), productID).
		Return(expectedProducts, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recommendations/"+productID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": productID.String()})

	rec := httptest.NewRecorder()
	handler.GetRecommendations(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var respBody map[string]interface{}
	err := json.NewDecoder(rec.Body).Decode(&respBody)
	require.NoError(t, err)

	assert.Equal(t, float64(1), respBody["total"])
	assert.NotNil(t, respBody["products"])
}

func TestGetRecommendations_InvalidUUID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUsecase := mockRecommendation.NewMockIRecommendationUsecase(ctrl)
	handler := recommendation.NewRecommendationService(mockUsecase)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recommendations/invalid-uuid", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid-uuid"})

	rec := httptest.NewRecorder()
	handler.GetRecommendations(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestGetRecommendations_UsecaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUsecase := mockRecommendation.NewMockIRecommendationUsecase(ctrl)
	handler := recommendation.NewRecommendationService(mockUsecase)

	productID := uuid.New()

	mockUsecase.EXPECT().
		GetRecommendations(gomock.Any(), productID).
		Return(nil, errors.New("something went wrong"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recommendations/"+productID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": productID.String()})

	rec := httptest.NewRecorder()
	handler.GetRecommendations(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- GetPersonalRecommendations (главная) ---

// Аноним: userID в контексте нет → usecase вызывается с uuid.Nil, personalized=false.
func TestGetPersonalRecommendations_Anonymous(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUsecase := mockRecommendation.NewMockIRecommendationUsecase(ctrl)
	handler := recommendation.NewRecommendationService(mockUsecase)

	products := []*models.Product{{ID: uuid.New(), Name: "Popular"}}
	mockUsecase.EXPECT().
		GetPersonalRecommendations(gomock.Any(), uuid.Nil).
		Return(products, false, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recommendation", nil)
	rec := httptest.NewRecorder()
	handler.GetPersonalRecommendations(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, false, body["personalized"])
	assert.Equal(t, float64(1), body["total"])
}

// Залогинен: userID берётся из контекста, personalized=true.
func TestGetPersonalRecommendations_Personalized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUsecase := mockRecommendation.NewMockIRecommendationUsecase(ctrl)
	handler := recommendation.NewRecommendationService(mockUsecase)

	userID := uuid.New()
	products := []*models.Product{{ID: uuid.New(), Name: "For you"}}
	mockUsecase.EXPECT().
		GetPersonalRecommendations(gomock.Any(), userID).
		Return(products, true, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recommendation", nil)
	ctx := context.WithValue(req.Context(), domains.UserIDKey{}, userID.String())
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.GetPersonalRecommendations(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, true, body["personalized"])
}

func TestGetPersonalRecommendations_UsecaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUsecase := mockRecommendation.NewMockIRecommendationUsecase(ctrl)
	handler := recommendation.NewRecommendationService(mockUsecase)

	mockUsecase.EXPECT().
		GetPersonalRecommendations(gomock.Any(), uuid.Nil).
		Return(nil, false, errors.New("boom"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recommendation", nil)
	rec := httptest.NewRecorder()
	handler.GetPersonalRecommendations(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
