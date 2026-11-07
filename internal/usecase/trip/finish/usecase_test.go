package finish_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/model"
	"github.com/505Eclypsia/template/internal/usecase/trip/finish"
)

func TestUseCase_Finish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMocktripRepository(ctrl)
	mockTx := NewMocktxManager(ctrl)
	mockMetrics := NewMockmetrics(ctrl)

	uc := finish.New(mockRepo, mockTx, mockMetrics)

	tripID := uuid.New()
	existing := &model.Trip{
		ID:     tripID,
		Status: domain.StatusActive,
	}

	mockRepo.EXPECT().GetTripByID(gomock.Any(), tripID).Return(existing, nil)
	mockTx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
	mockRepo.EXPECT().Finish(gomock.Any(), tripID, gomock.Any()).Return(nil)
	mockRepo.EXPECT().AddTripStatusHistory(gomock.Any(), gomock.Any()).Return(nil)
	mockMetrics.EXPECT().IncTripsCompleted(gomock.Any())

	res, err := uc.Finish(context.Background(), tripID, time.Now())
	if err != nil || res.Status != "completed" {
		t.Fatalf("failed finish: %v", err)
	}
}

func TestUseCase_Finish_AlreadyCompleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMocktripRepository(ctrl)
	mockTx := NewMocktxManager(ctrl)
	mockMetrics := NewMockmetrics(ctrl)
	uc := finish.New(mockRepo, mockTx, mockMetrics)

	tripID := uuid.New()
	completedTrip := &model.Trip{
		ID:     tripID,
		Status: domain.StatusCompleted,
	}

	mockRepo.EXPECT().GetTripByID(gomock.Any(), tripID).Return(completedTrip, nil)

	res, err := uc.Finish(context.Background(), tripID, time.Now())
	if err != domain.ErrTripCompleted {
		t.Fatalf("expected ErrTripCompleted, got %v", err)
	}
	if res != nil {
		t.Fatalf("expected nil result")
	}
}
