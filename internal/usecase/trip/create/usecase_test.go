package create_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/model"
	"github.com/505Eclypsia/template/internal/usecase/trip/create"
)

func TestUseCase_Create_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMocktripRepository(ctrl)
	mockTx := NewMocktxManager(ctrl)
	mockMetrics := NewMockmetrics(ctrl)

	uc := create.New(mockRepo, mockTx, mockMetrics)

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
	mockMetrics.EXPECT().IncTripsCreated(gomock.Any())

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

	uc := create.New(NewMocktripRepository(ctrl), NewMocktxManager(ctrl), NewMockmetrics(ctrl))

	trip := &model.Trip{Price: -100}
	_, _, err := uc.Create(context.Background(), trip, nil)
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
}

func TestUseCase_Create_DriverBusy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMocktripRepository(ctrl)
	mockTx := NewMocktxManager(ctrl)
	mockMetrics := NewMockmetrics(ctrl)
	uc := create.New(mockRepo, mockTx, mockMetrics)

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

	mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.ErrDriverBusy)

	res, isNew, err := uc.Create(context.Background(), trip, nil)
	if err != domain.ErrDriverBusy {
		t.Fatalf("expected ErrDriverBusy, got %v", err)
	}
	if isNew || res != nil {
		t.Fatalf("expected no trip result on error")
	}
}

func TestUseCase_Create_EmptyIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := create.New(NewMocktripRepository(ctrl), NewMocktxManager(ctrl), NewMockmetrics(ctrl))

	_, _, err := uc.Create(context.Background(), &model.Trip{Price: 100}, nil)

	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestUseCase_Create_IdempotencyKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMocktripRepository(ctrl)
	mockTx := NewMocktxManager(ctrl)
	mockMetrics := NewMockmetrics(ctrl)
	uc := create.New(mockRepo, mockTx, mockMetrics)

	keyID := uuid.New()
	userID, driverID := uuid.New(), uuid.New()
	newTrip := func(price int64) *model.Trip {
		return &model.Trip{
			UserID:         userID,
			DriverID:       driverID,
			StartLatitude:  59.93,
			StartLongitude: 30.31,
			EndLatitude:    59.92,
			EndLongitude:   30.36,
			Price:          price,
		}
	}

	mockTx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		Times(3)
	mockRepo.EXPECT().LockIdempotencyKey(gomock.Any(), keyID).Return(nil).Times(3)
	mockMetrics.EXPECT().IncTripsCreated(gomock.Any())

	var savedKey model.TripIdempotencyKey
	mockRepo.EXPECT().GetIdempotencyKeyByID(gomock.Any(), keyID).Return(nil, nil)
	mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().AddTripStatusHistory(gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().AddTripIdempotencyKey(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, k model.TripIdempotencyKey) error {
			savedKey = k
			return nil
		})

	first, isNew, err := uc.Create(context.Background(), newTrip(1000), &keyID)
	if err != nil || !isNew {
		t.Fatalf("first call: isNew=%v err=%v", isNew, err)
	}

	mockRepo.EXPECT().GetIdempotencyKeyByID(gomock.Any(), keyID).
		DoAndReturn(func(context.Context, uuid.UUID) (*model.TripIdempotencyKey, error) {
			return &savedKey, nil
		})
	mockRepo.EXPECT().GetTripByID(gomock.Any(), first.ID).Return(first, nil)

	replayed, isNew, err := uc.Create(context.Background(), newTrip(1000), &keyID)
	if err != nil || isNew {
		t.Fatalf("replay: isNew=%v err=%v", isNew, err)
	}
	if replayed.ID != first.ID {
		t.Fatalf("replay must return the same trip: %s != %s", replayed.ID, first.ID)
	}

	mockRepo.EXPECT().GetIdempotencyKeyByID(gomock.Any(), keyID).
		DoAndReturn(func(context.Context, uuid.UUID) (*model.TripIdempotencyKey, error) {
			return &savedKey, nil
		})

	res, isNew, err := uc.Create(context.Background(), newTrip(2000), &keyID)
	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("expected ErrIdempotencyConflict, got %v", err)
	}
	if res != nil || isNew {
		t.Fatalf("expected no trip on conflict")
	}
}
