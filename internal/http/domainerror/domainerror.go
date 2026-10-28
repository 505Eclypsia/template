package domainerror

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/generated/api"
)

func Problem(err error) (status int, body api.Problem) {
	var valErr *domain.ValidationError
	var jsonSyntaxErr *json.SyntaxError
	var jsonTypeErr *json.UnmarshalTypeError

	switch {
	// 400: Ошибки валидации входных данных
	case errors.As(err, &valErr):
		return http.StatusBadRequest, newProblem(
			http.StatusBadRequest,
			"invalid_request",
			"Invalid request",
			valErr.Error(),
		)

	case errors.As(err, &jsonSyntaxErr), errors.As(err, &jsonTypeErr), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return http.StatusBadRequest, newProblem(
			http.StatusBadRequest,
			"invalid_request",
			"Invalid request",
			"Failed to parse JSON body",
		)

	// 404: Поездка не найдена
	case errors.Is(err, domain.ErrTripNotFound):
		return http.StatusNotFound, newProblem(
			http.StatusNotFound,
			"trip_not_found",
			"Trip not found",
			"Trip was not found",
		)

	// 409: Поездка уже завершена
	case errors.Is(err, domain.ErrTripCompleted):
		return http.StatusConflict, newProblem(
			http.StatusConflict,
			"trip_completed",
			"Trip completed",
			"Operation is not allowed for a completed trip",
		)

	// 409: Водитель занят
	case errors.Is(err, domain.ErrDriverBusy):
		return http.StatusConflict, newProblem(
			http.StatusConflict,
			"driver_busy",
			"Driver busy",
			"Driver already has an active trip",
		)

	// 409: Идемпотентный ключ не найден
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return http.StatusConflict, newProblem(
			http.StatusConflict,
			"idempotency_conflict",
			"Idempotency conflict",
			"Idempotency conflict",
		)

	// 500: Все остальные непредвиденные / системные сбои
	default:
		return http.StatusInternalServerError, newProblem(
			http.StatusInternalServerError,
			"internal_error",
			"Internal server error",
			"Internal server error",
		)
	}
}

func WriteProblem(w http.ResponseWriter, r *http.Request, err error) {
	status, prob := Problem(err)

	instancePath := r.URL.Path
	prob.Instance = &instancePath

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(prob)
}

func newProblem(status int, code, title, detail string) api.Problem {
	typeURI := "https://tripgo.example/problems/" + code

	// #nosec G115
	return api.Problem{
		Type:   typeURI,
		Title:  title,
		Status: int32(status),
		Code:   code,
		Detail: &detail,
	}
}
