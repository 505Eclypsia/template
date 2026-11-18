package create

import (
	"github.com/505Eclypsia/template/internal/generated/api"
	"github.com/505Eclypsia/template/internal/model"
)

func convertJsonToModel(tripRB *api.CreateTripJSONRequestBody) model.Trip {
	trip := model.Trip{
		UserID:         tripRB.UserId,
		DriverID:       tripRB.DriverId,
		StartLatitude:  float64(tripRB.StartPoint.Latitude),
		StartLongitude: float64(tripRB.StartPoint.Longitude),
		EndLatitude:    float64(tripRB.EndPoint.Latitude),
		EndLongitude:   float64(tripRB.EndPoint.Longitude),
		Price:          tripRB.Price,
	}
	return trip
}

func convertModelToJson(tripModel *model.Trip) api.Trip {
	startPoint := api.Coordinates{Latitude: tripModel.StartLatitude, Longitude: tripModel.StartLongitude}
	endPoint := api.Coordinates{Latitude: tripModel.EndLatitude, Longitude: tripModel.EndLongitude}

	tripJSON := api.Trip{
		Id:         tripModel.ID,
		UserId:     tripModel.UserID,
		DriverId:   tripModel.DriverID,
		StartPoint: startPoint,
		EndPoint:   endPoint,
		Price:      tripModel.Price,
		Status:     api.TripStatus(tripModel.Status),
		StartedAt:  tripModel.StartedAt,
	}
	return tripJSON
}
