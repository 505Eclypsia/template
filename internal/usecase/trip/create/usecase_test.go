package create_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/505Eclypsia/template/internal/model"
	"github.com/505Eclypsia/template/internal/usecase/trip/create"
)

func TestUseCase_Create_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMocktripRepository(ctrl)
	mockTx := NewMocktxManager(ctrl)

	uc := create.New(mockRepo, mockTx)

	trip := &model.Trip{
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  59.93,
		StartLongitude: 30.31,
		EndLatitude:    59.92,
		EndLongitude:   30.36,
		Price:          1000,
	}

	mockTx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

	mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().AddTripStatusHistory(gomock.Any(), gomock.Any()).Return(nil)

	res, isNew, err := uc.Create(context.Background(), trip, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isNew || res == nil {
		t.Fatalf("expected new trip to be created")
	}
}

func TestUseCase_Create_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := create.New(NewMocktripRepository(ctrl), NewMocktxManager(ctrl))

	// Отрицательная цена
	trip := &model.Trip{Price: -100}
	_, _, err := uc.Create(context.Background(), trip, nil)
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
}
