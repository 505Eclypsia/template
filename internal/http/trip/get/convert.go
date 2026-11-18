package get

import (
	"github.com/505Eclypsia/template/internal/generated/api"
	"github.com/505Eclypsia/template/internal/model"
)

func convertModelToJSON(tripModel *model.Trip) api.Trip {
	return api.Trip{
		Id:         tripModel.ID,
		UserId:     tripModel.UserID,
		DriverId:   tripModel.DriverID,
		StartPoint: api.Coordinates{Latitude: tripModel.StartLatitude, Longitude: tripModel.StartLongitude},
		EndPoint:   api.Coordinates{Latitude: tripModel.EndLatitude, Longitude: tripModel.EndLongitude},
		Price:      tripModel.Price,
		Status:     api.TripStatus(tripModel.Status),
		StartedAt:  tripModel.StartedAt,
		FinishedAt: tripModel.FinishedAt,
	}
}
