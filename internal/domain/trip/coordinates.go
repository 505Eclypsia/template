package trip

import "math"

const errCoordsOutOfRange = "coordinates out of range"

func ValidateCoordinates(prefix string, latitude, longitude float64) error {
	switch {
	case math.IsNaN(latitude) || latitude < -90 || latitude > 90:
		return &ValidationError{Field: prefix + ".latitude", Message: errCoordsOutOfRange}
	case math.IsNaN(longitude) || longitude < -180 || longitude > 180:
		return &ValidationError{Field: prefix + ".longitude", Message: errCoordsOutOfRange}
	}
	return nil
}
