package tests

import (
	"context"
	"errors"
	"testing"

	recommendationRepo "github.com/go-park-mail-ru/2025_1_ChillGuys/internal/infrastructure/repository/postgres/mocks"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/models"
	mockProduct "github.com/go-park-mail-ru/2025_1_ChillGuys/internal/usecase/mocks"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/usecase/recommendation"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// --- GetRecommendations (item-to-item: co-purchase + контентный добор) ---

// co-purchase заполняет часть, остальное добирается контентом; дубли и сам товар исключаются.
func TestGetRecommendations_CoPurchaseAndContentFill(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := recommendationRepo.NewMockIRecommendationRepository(ctrl)
	pu := mockProduct.NewMockIProductUsecase(ctrl)
	usecase := recommendation.NewRecommendationUsecase(pu, repo)

	productID := uuid.New()
	cp1, cp2 := uuid.New(), uuid.New()
	subcat := uuid.New()
	c1, c2 := uuid.New(), uuid.New()

	coPurchased := []uuid.UUID{cp1, cp2}
	// content возвращает дубль cp1, сам productID (оба должны отсеяться) и новые c1, c2
	contentIDs := []uuid.UUID{cp1, productID, c1, c2}
	expectedIDs := []uuid.UUID{cp1, cp2, c1, c2} // порядок: co-purchase первым

	expectedProducts := []*models.Product{{ID: cp1}, {ID: cp2}, {ID: c1}, {ID: c2}}

	repo.EXPECT().GetCoPurchasedProductIDs(gomock.Any(), productID, 10).Return(coPurchased, nil)
	repo.EXPECT().GetCategoryIDsByProductID(gomock.Any(), productID).Return([]uuid.UUID{subcat}, nil)
	repo.EXPECT().GetProductIDsBySubcategoryID(gomock.Any(), subcat, 10).Return(contentIDs, nil)
	pu.EXPECT().GetProductsByIDs(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, ids []uuid.UUID) ([]*models.Product, error) {
			assert.Equal(t, expectedIDs, ids)
			return expectedProducts, nil
		})

	products, err := usecase.GetRecommendations(context.Background(), productID)
	assert.NoError(t, err)
	assert.Equal(t, expectedProducts, products)
}

// co-purchase заполняет лимит целиком → контентные методы не вызываются.
func TestGetRecommendations_CoPurchaseFillsLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := recommendationRepo.NewMockIRecommendationRepository(ctrl)
	pu := mockProduct.NewMockIProductUsecase(ctrl)
	usecase := recommendation.NewRecommendationUsecase(pu, repo)

	productID := uuid.New()
	coPurchased := make([]uuid.UUID, 10)
	for i := range coPurchased {
		coPurchased[i] = uuid.New()
	}

	repo.EXPECT().GetCoPurchasedProductIDs(gomock.Any(), productID, 10).Return(coPurchased, nil)
	pu.EXPECT().GetProductsByIDs(gomock.Any(), coPurchased).Return([]*models.Product{}, nil)

	_, err := usecase.GetRecommendations(context.Background(), productID)
	assert.NoError(t, err)
}

func TestGetRecommendations_CoPurchaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := recommendationRepo.NewMockIRecommendationRepository(ctrl)
	pu := mockProduct.NewMockIProductUsecase(ctrl)
	usecase := recommendation.NewRecommendationUsecase(pu, repo)

	productID := uuid.New()
	repo.EXPECT().GetCoPurchasedProductIDs(gomock.Any(), productID, 10).Return(nil, errors.New("db error"))

	products, err := usecase.GetRecommendations(context.Background(), productID)
	assert.Error(t, err)
	assert.Nil(t, products)
}

// нет co-purchase (пустые данные) → всё строится на контенте.
func TestGetRecommendations_ContentOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := recommendationRepo.NewMockIRecommendationRepository(ctrl)
	pu := mockProduct.NewMockIProductUsecase(ctrl)
	usecase := recommendation.NewRecommendationUsecase(pu, repo)

	productID := uuid.New()
	subcat := uuid.New()
	c1 := uuid.New()

	repo.EXPECT().GetCoPurchasedProductIDs(gomock.Any(), productID, 10).Return(nil, nil)
	repo.EXPECT().GetCategoryIDsByProductID(gomock.Any(), productID).Return([]uuid.UUID{subcat}, nil)
	repo.EXPECT().GetProductIDsBySubcategoryID(gomock.Any(), subcat, 10).Return([]uuid.UUID{c1}, nil)
	pu.EXPECT().GetProductsByIDs(gomock.Any(), []uuid.UUID{c1}).Return([]*models.Product{{ID: c1}}, nil)

	products, err := usecase.GetRecommendations(context.Background(), productID)
	assert.NoError(t, err)
	assert.Len(t, products, 1)
}

func TestGetRecommendations_GetCategoryIDsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := recommendationRepo.NewMockIRecommendationRepository(ctrl)
	pu := mockProduct.NewMockIProductUsecase(ctrl)
	usecase := recommendation.NewRecommendationUsecase(pu, repo)

	productID := uuid.New()
	repo.EXPECT().GetCoPurchasedProductIDs(gomock.Any(), productID, 10).Return(nil, nil)
	repo.EXPECT().GetCategoryIDsByProductID(gomock.Any(), productID).Return(nil, errors.New("db error"))

	products, err := usecase.GetRecommendations(context.Background(), productID)
	assert.Error(t, err)
	assert.Nil(t, products)
}

// вообще ничего не нашлось → nil, nil (без вызова GetProductsByIDs).
func TestGetRecommendations_NoProductsFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := recommendationRepo.NewMockIRecommendationRepository(ctrl)
	pu := mockProduct.NewMockIProductUsecase(ctrl)
	usecase := recommendation.NewRecommendationUsecase(pu, repo)

	productID := uuid.New()
	subcat := uuid.New()

	repo.EXPECT().GetCoPurchasedProductIDs(gomock.Any(), productID, 10).Return(nil, nil)
	repo.EXPECT().GetCategoryIDsByProductID(gomock.Any(), productID).Return([]uuid.UUID{subcat}, nil)
	repo.EXPECT().GetProductIDsBySubcategoryID(gomock.Any(), subcat, 10).Return([]uuid.UUID{}, nil)

	products, err := usecase.GetRecommendations(context.Background(), productID)
	assert.NoError(t, err)
	assert.Nil(t, products)
}

func TestGetRecommendations_ProductUsecaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := recommendationRepo.NewMockIRecommendationRepository(ctrl)
	pu := mockProduct.NewMockIProductUsecase(ctrl)
	usecase := recommendation.NewRecommendationUsecase(pu, repo)

	productID := uuid.New()
	x := uuid.New()

	repo.EXPECT().GetCoPurchasedProductIDs(gomock.Any(), productID, 10).Return([]uuid.UUID{x}, nil)
	repo.EXPECT().GetCategoryIDsByProductID(gomock.Any(), productID).Return(nil, nil)
	pu.EXPECT().GetProductsByIDs(gomock.Any(), []uuid.UUID{x}).Return(nil, errors.New("usecase error"))

	products, err := usecase.GetRecommendations(context.Background(), productID)
	assert.Error(t, err)
	assert.Nil(t, products)
}

// --- GetPersonalRecommendations (главная: персонально / популярное) ---

// аноним (userID == Nil) → сразу популярное, personalized=false.
func TestGetPersonalRecommendations_Anonymous(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := recommendationRepo.NewMockIRecommendationRepository(ctrl)
	pu := mockProduct.NewMockIProductUsecase(ctrl)
	usecase := recommendation.NewRecommendationUsecase(pu, repo)

	popular := []uuid.UUID{uuid.New(), uuid.New()}
	repo.EXPECT().GetPopularProductIDs(gomock.Any(), 20).Return(popular, nil)
	pu.EXPECT().GetProductsByIDs(gomock.Any(), popular).Return([]*models.Product{{ID: popular[0]}, {ID: popular[1]}}, nil)

	products, personalized, err := usecase.GetPersonalRecommendations(context.Background(), uuid.Nil)
	assert.NoError(t, err)
	assert.False(t, personalized)
	assert.Len(t, products, 2)
}

// есть покупки → комплементарные товары («с этим покупают»), personalized=true.
func TestGetPersonalRecommendations_Complementary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := recommendationRepo.NewMockIRecommendationRepository(ctrl)
	pu := mockProduct.NewMockIProductUsecase(ctrl)
	usecase := recommendation.NewRecommendationUsecase(pu, repo)

	userID := uuid.New()
	bought1, bought2 := uuid.New(), uuid.New()
	comp1, comp2 := uuid.New(), uuid.New()

	repo.EXPECT().GetPurchasedProductIDs(gomock.Any(), userID).
		Return([]uuid.UUID{bought1, bought2}, nil)
	repo.EXPECT().GetComplementaryProductIDs(gomock.Any(), []uuid.UUID{bought1, bought2}, 20).
		Return([]uuid.UUID{comp1, comp2}, nil)
	// комплементарных 2 < 20 → добор по подкатегориям, но их нет
	repo.EXPECT().GetPreferredSubcategoryIDs(gomock.Any(), userID, 5).Return(nil, nil)
	pu.EXPECT().GetProductsByIDs(gomock.Any(), []uuid.UUID{comp1, comp2}).
		Return([]*models.Product{{ID: comp1}, {ID: comp2}}, nil)

	products, personalized, err := usecase.GetPersonalRecommendations(context.Background(), userID)
	assert.NoError(t, err)
	assert.True(t, personalized)
	assert.Len(t, products, 2)
}

// комплементарных нет → добор из любимых подкатегорий чередуется (round-robin).
func TestGetPersonalRecommendations_TopupRoundRobin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := recommendationRepo.NewMockIRecommendationRepository(ctrl)
	pu := mockProduct.NewMockIProductUsecase(ctrl)
	usecase := recommendation.NewRecommendationUsecase(pu, repo)

	userID := uuid.New()
	bought := uuid.New()
	phones, laptops := uuid.New(), uuid.New()
	pa1, pa2 := uuid.New(), uuid.New()
	lb1, lb2 := uuid.New(), uuid.New()

	repo.EXPECT().GetPurchasedProductIDs(gomock.Any(), userID).Return([]uuid.UUID{bought}, nil)
	repo.EXPECT().GetComplementaryProductIDs(gomock.Any(), []uuid.UUID{bought}, 20).Return(nil, nil)
	repo.EXPECT().GetPreferredSubcategoryIDs(gomock.Any(), userID, 5).
		Return([]uuid.UUID{phones, laptops}, nil)
	repo.EXPECT().GetProductIDsBySubcategoryID(gomock.Any(), phones, 20).
		Return([]uuid.UUID{pa1, pa2}, nil)
	repo.EXPECT().GetProductIDsBySubcategoryID(gomock.Any(), laptops, 20).
		Return([]uuid.UUID{lb1, lb2}, nil)

	expected := []uuid.UUID{pa1, lb1, pa2, lb2} // чередование
	pu.EXPECT().GetProductsByIDs(gomock.Any(), expected).
		Return([]*models.Product{{ID: pa1}, {ID: lb1}, {ID: pa2}, {ID: lb2}}, nil)

	products, personalized, err := usecase.GetPersonalRecommendations(context.Background(), userID)
	assert.NoError(t, err)
	assert.True(t, personalized)
	assert.Len(t, products, 4)
}

// нет покупок → cold-start → популярное, personalized=false.
func TestGetPersonalRecommendations_ColdStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := recommendationRepo.NewMockIRecommendationRepository(ctrl)
	pu := mockProduct.NewMockIProductUsecase(ctrl)
	usecase := recommendation.NewRecommendationUsecase(pu, repo)

	userID := uuid.New()
	popular := []uuid.UUID{uuid.New()}

	repo.EXPECT().GetPurchasedProductIDs(gomock.Any(), userID).Return(nil, nil)
	repo.EXPECT().GetPopularProductIDs(gomock.Any(), 20).Return(popular, nil)
	pu.EXPECT().GetProductsByIDs(gomock.Any(), popular).Return([]*models.Product{{ID: popular[0]}}, nil)

	products, personalized, err := usecase.GetPersonalRecommendations(context.Background(), userID)
	assert.NoError(t, err)
	assert.False(t, personalized)
	assert.Len(t, products, 1)
}

// комплементарных нет и все кандидаты из подкатегорий уже куплены → популярное.
func TestGetPersonalRecommendations_AllExcluded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := recommendationRepo.NewMockIRecommendationRepository(ctrl)
	pu := mockProduct.NewMockIProductUsecase(ctrl)
	usecase := recommendation.NewRecommendationUsecase(pu, repo)

	userID := uuid.New()
	subcat := uuid.New()
	p1, p2 := uuid.New(), uuid.New()
	popular := []uuid.UUID{uuid.New()}

	repo.EXPECT().GetPurchasedProductIDs(gomock.Any(), userID).Return([]uuid.UUID{p1, p2}, nil)
	repo.EXPECT().GetComplementaryProductIDs(gomock.Any(), []uuid.UUID{p1, p2}, 20).Return(nil, nil)
	repo.EXPECT().GetPreferredSubcategoryIDs(gomock.Any(), userID, 5).Return([]uuid.UUID{subcat}, nil)
	repo.EXPECT().GetProductIDsBySubcategoryID(gomock.Any(), subcat, 20).Return([]uuid.UUID{p1, p2}, nil)
	repo.EXPECT().GetPopularProductIDs(gomock.Any(), 20).Return(popular, nil)
	pu.EXPECT().GetProductsByIDs(gomock.Any(), popular).Return([]*models.Product{{ID: popular[0]}}, nil)

	products, personalized, err := usecase.GetPersonalRecommendations(context.Background(), userID)
	assert.NoError(t, err)
	assert.False(t, personalized)
	assert.Len(t, products, 1)
}

func TestGetPersonalRecommendations_PurchasedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := recommendationRepo.NewMockIRecommendationRepository(ctrl)
	pu := mockProduct.NewMockIProductUsecase(ctrl)
	usecase := recommendation.NewRecommendationUsecase(pu, repo)

	userID := uuid.New()
	repo.EXPECT().GetPurchasedProductIDs(gomock.Any(), userID).Return(nil, errors.New("db error"))

	products, personalized, err := usecase.GetPersonalRecommendations(context.Background(), userID)
	assert.Error(t, err)
	assert.False(t, personalized)
	assert.Nil(t, products)
}

func TestGetPersonalRecommendations_ComplementaryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := recommendationRepo.NewMockIRecommendationRepository(ctrl)
	pu := mockProduct.NewMockIProductUsecase(ctrl)
	usecase := recommendation.NewRecommendationUsecase(pu, repo)

	userID := uuid.New()
	bought := uuid.New()
	repo.EXPECT().GetPurchasedProductIDs(gomock.Any(), userID).Return([]uuid.UUID{bought}, nil)
	repo.EXPECT().GetComplementaryProductIDs(gomock.Any(), []uuid.UUID{bought}, 20).
		Return(nil, errors.New("boom"))

	products, personalized, err := usecase.GetPersonalRecommendations(context.Background(), userID)
	assert.Error(t, err)
	assert.False(t, personalized)
	assert.Nil(t, products)
}
