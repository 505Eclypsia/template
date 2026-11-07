package get_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/model"
	"github.com/505Eclypsia/template/internal/usecase/trip/get"
)

func TestUseCase_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMocktripRepository(ctrl)
	uc := get.New(mockRepo)

	tripID := uuid.New()
	expected := &model.Trip{ID: tripID}

	mockRepo.EXPECT().GetTripByID(gomock.Any(), tripID).Return(expected, nil)

	res, err := uc.Get(context.Background(), tripID)
	if err != nil || res.ID != tripID {
		t.Fatalf("unexpected result: %v, err: %v", res, err)
	}

	// 404
	mockRepo.EXPECT().GetTripByID(gomock.Any(), gomock.Any()).Return(nil, domain.ErrTripNotFound)
	_, err = uc.Get(context.Background(), uuid.New())
	if err != domain.ErrTripNotFound {
		t.Fatalf("expected 404 error, got: %v", err)
	}
}
