package tests

import (
	"context"

	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-park-mail-ru/2025_1_ChillGuys/internal/infrastructure/repository/postgres/recommendation"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGetCategoryIDsByProductID_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	productID := uuid.New()
	expectedCategories := []uuid.UUID{
		uuid.New(),
		uuid.New(),
	}

	rows := sqlmock.NewRows([]string{"subcategory_id"}).
		AddRow(expectedCategories[0]).
		AddRow(expectedCategories[1])

	mock.ExpectQuery("SELECT subcategory_id FROM bazaar.product_subcategory WHERE product_id = \\$1").
		WithArgs(productID).
		WillReturnRows(rows)

	repo := recommendation.NewRecommendationRepository(db)
	result, err := repo.GetCategoryIDsByProductID(context.Background(), productID)

	assert.NoError(t, err)
	assert.Equal(t, expectedCategories, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCategoryIDsByProductID_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	productID := uuid.New()

	rows := sqlmock.NewRows([]string{"subcategory_id"})

	mock.ExpectQuery("SELECT subcategory_id FROM bazaar.product_subcategory WHERE product_id = \\$1").
		WithArgs(productID).
		WillReturnRows(rows)

	repo := recommendation.NewRecommendationRepository(db)
	result, err := repo.GetCategoryIDsByProductID(context.Background(), productID)

	assert.NoError(t, err)
	assert.Empty(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCategoryIDsByProductID_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	productID := uuid.New()

	mock.ExpectQuery("SELECT subcategory_id FROM bazaar.product_subcategory WHERE product_id = \\$1").
		WithArgs(productID).
		WillReturnError(errors.New("database error"))

	repo := recommendation.NewRecommendationRepository(db)
	_, err = repo.GetCategoryIDsByProductID(context.Background(), productID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCategoryIDsByProductID_ScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	productID := uuid.New()

	rows := sqlmock.NewRows([]string{"subcategory_id"}).
		AddRow("invalid-uuid")

	mock.ExpectQuery("SELECT subcategory_id FROM bazaar.product_subcategory WHERE product_id = \\$1").
		WithArgs(productID).
		WillReturnRows(rows)

	repo := recommendation.NewRecommendationRepository(db)
	_, err = repo.GetCategoryIDsByProductID(context.Background(), productID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UUID length")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProductIDsBySubcategoryID_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	subcategoryID := uuid.New()
	expectedProducts := []uuid.UUID{
		uuid.New(),
		uuid.New(),
	}

	rows := sqlmock.NewRows([]string{"product_id"}).
		AddRow(expectedProducts[0]).
		AddRow(expectedProducts[1])

	mock.ExpectQuery("bazaar\\.product_subcategory ps").
		WithArgs(subcategoryID, 20, 10).
		WillReturnRows(rows)

	repo := recommendation.NewRecommendationRepository(db)
	result, err := repo.GetProductIDsBySubcategoryID(context.Background(), subcategoryID, 10)

	assert.NoError(t, err)
	assert.Equal(t, expectedProducts, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProductIDsBySubcategoryID_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	subcategoryID := uuid.New()

	rows := sqlmock.NewRows([]string{"product_id"})

	mock.ExpectQuery("bazaar\\.product_subcategory ps").
		WithArgs(subcategoryID, 20, 10).
		WillReturnRows(rows)

	repo := recommendation.NewRecommendationRepository(db)
	result, err := repo.GetProductIDsBySubcategoryID(context.Background(), subcategoryID, 10)

	assert.NoError(t, err)
	assert.Empty(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProductIDsBySubcategoryID_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	subcategoryID := uuid.New()

	mock.ExpectQuery("bazaar\\.product_subcategory ps").
		WithArgs(subcategoryID, 20, 10).
		WillReturnError(errors.New("database error"))

	repo := recommendation.NewRecommendationRepository(db)
	_, err = repo.GetProductIDsBySubcategoryID(context.Background(), subcategoryID, 10)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProductIDsBySubcategoryID_ScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	subcategoryID := uuid.New()

	rows := sqlmock.NewRows([]string{"product_id"}).
		AddRow("invalid-uuid")

	mock.ExpectQuery("bazaar\\.product_subcategory ps").
		WithArgs(subcategoryID, 20, 10).
		WillReturnRows(rows)

	repo := recommendation.NewRecommendationRepository(db)
	_, err = repo.GetProductIDsBySubcategoryID(context.Background(), subcategoryID, 10)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UUID length")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCoPurchasedProductIDs_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %s", err)
	}
	defer db.Close()

	productID := uuid.New()
	expected := []uuid.UUID{uuid.New(), uuid.New()}

	rows := sqlmock.NewRows([]string{"product_id"}).
		AddRow(expected[0]).
		AddRow(expected[1])

	mock.ExpectQuery("bazaar\\.order_item oi1").
		WithArgs(productID, 10).
		WillReturnRows(rows)

	repo := recommendation.NewRecommendationRepository(db)
	result, err := repo.GetCoPurchasedProductIDs(context.Background(), productID, 10)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCoPurchasedProductIDs_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %s", err)
	}
	defer db.Close()

	productID := uuid.New()

	mock.ExpectQuery("bazaar\\.order_item oi1").
		WithArgs(productID, 10).
		WillReturnError(errors.New("database error"))

	repo := recommendation.NewRecommendationRepository(db)
	_, err = repo.GetCoPurchasedProductIDs(context.Background(), productID, 10)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPreferredSubcategoryIDs_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %s", err)
	}
	defer db.Close()

	userID := uuid.New()
	expected := []uuid.UUID{uuid.New(), uuid.New()}

	rows := sqlmock.NewRows([]string{"subcategory_id"}).
		AddRow(expected[0]).
		AddRow(expected[1])

	mock.ExpectQuery("GROUP BY ps\\.subcategory_id").
		WithArgs(userID, 5).
		WillReturnRows(rows)

	repo := recommendation.NewRecommendationRepository(db)
	result, err := repo.GetPreferredSubcategoryIDs(context.Background(), userID, 5)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPurchasedProductIDs_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %s", err)
	}
	defer db.Close()

	userID := uuid.New()
	expected := []uuid.UUID{uuid.New()}

	rows := sqlmock.NewRows([]string{"product_id"}).AddRow(expected[0])

	mock.ExpectQuery("SELECT DISTINCT oi\\.product_id").
		WithArgs(userID).
		WillReturnRows(rows)

	repo := recommendation.NewRecommendationRepository(db)
	result, err := repo.GetPurchasedProductIDs(context.Background(), userID)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPopularProductIDs_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %s", err)
	}
	defer db.Close()

	expected := []uuid.UUID{uuid.New(), uuid.New()}

	rows := sqlmock.NewRows([]string{"id"}).
		AddRow(expected[0]).
		AddRow(expected[1])

	mock.ExpectQuery("bazaar\\.product p\\b").
		WithArgs(20, 20).
		WillReturnRows(rows)

	repo := recommendation.NewRecommendationRepository(db)
	result, err := repo.GetPopularProductIDs(context.Background(), 20)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetComplementaryProductIDs_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %s", err)
	}
	defer db.Close()

	purchased := []uuid.UUID{uuid.New(), uuid.New()}
	expected := []uuid.UUID{uuid.New(), uuid.New()}

	rows := sqlmock.NewRows([]string{"product_id"}).
		AddRow(expected[0]).
		AddRow(expected[1])

	mock.ExpectQuery("ANY\\(\\$1::uuid\\[\\]\\)").
		WithArgs(sqlmock.AnyArg(), 20).
		WillReturnRows(rows)

	repo := recommendation.NewRecommendationRepository(db)
	result, err := repo.GetComplementaryProductIDs(context.Background(), purchased, 20)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetComplementaryProductIDs_EmptyInput(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %s", err)
	}
	defer db.Close()

	repo := recommendation.NewRecommendationRepository(db)
	result, err := repo.GetComplementaryProductIDs(context.Background(), nil, 20)

	assert.NoError(t, err)
	assert.Nil(t, result)
}
