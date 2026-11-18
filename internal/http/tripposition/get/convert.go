package get

import (
	"github.com/505Eclypsia/template/internal/generated/api"
	"github.com/505Eclypsia/template/internal/model"
)

func convertModelsToJSON(tripPositions []model.TripPosition) api.TripPositionsResponse {
	positions := make([]api.TripPosition, 0, len(tripPositions))
	for _, p := range tripPositions {
		positions = append(positions, api.TripPosition{
			Id:          p.ID,
			TripId:      p.TripID,
			Coordinates: api.Coordinates{Latitude: p.Latitude, Longitude: p.Longitude},
			RecordedAt:  p.RecordedAt,
		})
	}
	return api.TripPositionsResponse{Positions: positions}
}
