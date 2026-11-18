package push_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/505Eclypsia/template/internal/gateway/push"
)

func TestClient_RequestPosition_Success(t *testing.T) {
	driverID := uuid.New()
	tripID := uuid.New()

	var gotMethod, gotPath string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	client := push.New(srv.URL, time.Second)
	defer client.Close()

	err := client.RequestPosition(context.Background(), driverID, tripID)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/api/v1/push", gotPath)
	assert.Equal(t, driverID.String(), gotBody["recipient_id"])
	assert.Equal(t, "REQUEST_POSITION", gotBody["kind"])
	data, ok := gotBody["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, tripID.String(), data["trip_id"])
}

func TestClient_RequestPosition_StatusErrors(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		wantClient bool
		wantServer bool
	}{
		{name: "4xx client error", status: http.StatusBadRequest, wantClient: true},
		{name: "5xx server error", status: http.StatusServiceUnavailable, wantServer: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"error":"failed"}`))
			}))
			defer srv.Close()

			client := push.New(srv.URL, time.Second)
			defer client.Close()

			err := client.RequestPosition(context.Background(), uuid.New(), uuid.New())
			var statusErr *push.StatusError
			require.ErrorAs(t, err, &statusErr)
			assert.Equal(t, tt.status, statusErr.StatusCode)
			assert.Equal(t, tt.wantClient, statusErr.ClientError())
			assert.Equal(t, tt.wantServer, statusErr.ServerError())
			assert.False(t, errors.Is(err, push.ErrTransport))
		})
	}
}

func TestClient_RequestPosition_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(200 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	client := push.New(srv.URL, 20*time.Millisecond)
	defer client.Close()

	err := client.RequestPosition(context.Background(), uuid.New(), uuid.New())
	require.ErrorIs(t, err, push.ErrTransport)
}
