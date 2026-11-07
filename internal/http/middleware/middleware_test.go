package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/505Eclypsia/template/internal/http/middleware"
)

func newRouter(h http.HandlerFunc) (*chi.Mux, *bytes.Buffer) {
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))

	r := chi.NewRouter()
	middleware.Chain(r, logger)
	r.Get("/trips/{id}", h)
	return r, logs
}

func logEntries(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()

	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		if line == "" {
			continue
		}
		var e map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &e))
		entries = append(entries, e)
	}
	return entries
}

func findEntry(t *testing.T, entries []map[string]any, msg string) map[string]any {
	t.Helper()

	for _, e := range entries {
		if e["msg"] == msg {
			return e
		}
	}
	require.Failf(t, "log entry not found", "msg=%q entries=%v", msg, entries)
	return nil
}

func TestChain_UsesRoutePattern(t *testing.T) {
	r, logs := newRouter(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, id := range []string{"aaa", "bbb"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trips/"+id, nil))
		require.Equal(t, http.StatusOK, rec.Code)
	}

	for _, e := range logEntries(t, logs) {
		assert.Equal(t, "request handled", e["msg"])
		assert.Equal(t, "/trips/{id}", e["route"])
		assert.EqualValues(t, http.StatusOK, e["status"])
	}
}

func TestChain_UnknownRouteIsNotHighCardinality(t *testing.T) {
	r, logs := newRouter(func(http.ResponseWriter, *http.Request) {})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/no/such/path/12345", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)

	e := findEntry(t, logEntries(t, logs), "request handled")
	assert.Equal(t, "unmatched", e["route"])
}

func TestChain_PanicIsLoggedAndReturns500(t *testing.T) {
	r, logs := newRouter(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trips/42", nil))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))
	assert.NotContains(t, rec.Body.String(), "boom")

	entries := logEntries(t, logs)
	findEntry(t, entries, "panic recovered")

	handled := findEntry(t, entries, "request handled")
	assert.Equal(t, "ERROR", handled["level"])
	assert.EqualValues(t, http.StatusInternalServerError, handled["status"])
	assert.Equal(t, "/trips/{id}", handled["route"])
	assert.Equal(t, "boom", handled["panic"])
}

func TestChain_ServerErrorIsLoggedAsError(t *testing.T) {
	r, logs := newRouter(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trips/1", nil))

	e := findEntry(t, logEntries(t, logs), "request handled")
	assert.Equal(t, "ERROR", e["level"])
}
