package create

import (
	"github.com/505Eclypsia/template/internal/generated/api"
	"github.com/505Eclypsia/template/internal/model"
)

func convertJSONToModel(req *api.CreateTripPositionJSONRequestBody, tripID api.TripId) model.TripPosition {
	return model.TripPosition{
		TripID:     tripID,
		Latitude:   req.Coordinates.Latitude,
		Longitude:  req.Coordinates.Longitude,
		RecordedAt: req.RecordedAt,
	}
}

func convertModelToJSON(tripPosition *model.TripPosition) api.TripPosition {
	return api.TripPosition{
		Id:          tripPosition.ID,
		TripId:      tripPosition.TripID,
		Coordinates: api.Coordinates{Latitude: tripPosition.Latitude, Longitude: tripPosition.Longitude},
		RecordedAt:  tripPosition.RecordedAt,
	}
}
