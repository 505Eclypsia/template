package positions

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/505Eclypsia/template/internal/model"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type fakeTicker struct{ ch chan time.Time }

func newFakeTicker() *fakeTicker          { return &fakeTicker{ch: make(chan time.Time)} }
func (f *fakeTicker) C() <-chan time.Time { return f.ch }
func (f *fakeTicker) Stop()               {}
func (f *fakeTicker) tick(t *testing.T)   { t.Helper(); f.ch <- time.Now() }

type fakeRepo struct {
	mu     sync.Mutex
	trips  []model.StaleTrip
	err    error
	called chan struct{}
}

func newFakeRepo(trips ...model.StaleTrip) *fakeRepo {
	return &fakeRepo{trips: trips, called: make(chan struct{}, 100)}
}

func (r *fakeRepo) ListStaleTrips(context.Context, time.Duration) ([]model.StaleTrip, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.called <- struct{}{}
	return append([]model.StaleTrip(nil), r.trips...), r.err
}

type fakePusher struct {
	fn func(ctx context.Context, driverID, tripID uuid.UUID) error

	mu    sync.Mutex
	calls []uuid.UUID
}

func (p *fakePusher) RequestPosition(ctx context.Context, driverID, tripID uuid.UUID) error {
	p.mu.Lock()
	p.calls = append(p.calls, tripID)
	p.mu.Unlock()
	if p.fn != nil {
		return p.fn(ctx, driverID, tripID)
	}
	return nil
}

func (p *fakePusher) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

type fakeMetrics struct {
	mu       sync.Mutex
	checks   map[string]int
	requests map[string]int
}

func newFakeMetrics() *fakeMetrics {
	return &fakeMetrics{checks: map[string]int{}, requests: map[string]int{}}
}

func (m *fakeMetrics) IncStaleCheck(_ context.Context, result string) {
	m.mu.Lock()
	m.checks[result]++
	m.mu.Unlock()
}

func (m *fakeMetrics) IncPositionRequest(_ context.Context, result string) {
	m.mu.Lock()
	m.requests[result]++
	m.mu.Unlock()
}

func (m *fakeMetrics) RecordPositionRequestDuration(context.Context, time.Duration, string) {}
func (m *fakeMetrics) AddQueueSize(context.Context, int64)                                  {}
func (m *fakeMetrics) AddActiveWorkers(context.Context, int64)                              {}

func (m *fakeMetrics) check(result string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.checks[result]
}

func (m *fakeMetrics) request(result string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requests[result]
}

func staleTrips(n int) []model.StaleTrip {
	trips := make([]model.StaleTrip, n)
	for i := range trips {
		trips[i] = model.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}
	}
	return trips
}

func newWorker(t *testing.T, cfg Config, repo *fakeRepo, p *fakePusher, m *fakeMetrics) (*Worker, *fakeTicker) {
	t.Helper()
	ticker := newFakeTicker()
	w, err := New(cfg, repo, p, m, ticker, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	return w, ticker
}

func defaultCfg() Config {
	return Config{StaleAfter: time.Minute, Workers: 2, QueueSize: 10, RequestTimeout: time.Second}
}

func stopWithTimeout(t *testing.T, w *Worker) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, w.Stop(ctx))
}

func TestScan_AlreadyQueuedTripIsNotQueuedTwice(t *testing.T) {
	trips := staleTrips(1)
	m := newFakeMetrics()
	w, _ := newWorker(t, defaultCfg(), newFakeRepo(trips...), &fakePusher{}, m)

	w.scan(context.Background())
	w.scan(context.Background())

	assert.Equal(t, 1, m.check(CheckQueued))
	assert.Equal(t, 1, m.check(CheckDuplicate))
	assert.Len(t, w.queue, 1)
}

func TestProcess_CancelBetweenEnqueueAndProcessSkipsPush(t *testing.T) {
	trips := staleTrips(1)
	p := &fakePusher{}
	m := newFakeMetrics()
	w, _ := newWorker(t, defaultCfg(), newFakeRepo(trips...), p, m)

	w.scan(context.Background())
	w.Cancel(trips[0].TripID)

	w.Start(context.Background())
	stopWithTimeout(t, w)

	assert.Equal(t, 0, p.callCount())
	assert.Equal(t, 1, m.request(RequestCancelled))
}

func TestProcess_CancelThenRequeueSendsOnlyOnce(t *testing.T) {
	trips := staleTrips(1)
	p := &fakePusher{}
	m := newFakeMetrics()
	w, _ := newWorker(t, Config{StaleAfter: time.Minute, Workers: 1, QueueSize: 10, RequestTimeout: time.Second},
		newFakeRepo(trips...), p, m)

	w.scan(context.Background())
	w.Cancel(trips[0].TripID)
	w.scan(context.Background())

	w.Start(context.Background())
	stopWithTimeout(t, w)

	assert.Equal(t, 1, p.callCount())
	assert.Equal(t, 1, m.request(RequestSuccess))
	assert.Equal(t, 1, m.request(RequestCancelled))
}

func TestScan_FullQueueDropsInsteadOfBlocking(t *testing.T) {
	cfg := Config{StaleAfter: time.Minute, Workers: 1, QueueSize: 2, RequestTimeout: time.Second}
	m := newFakeMetrics()
	w, _ := newWorker(t, cfg, newFakeRepo(staleTrips(5)...), &fakePusher{}, m)

	done := make(chan struct{})
	go func() {
		w.scan(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scan blocked on a full queue")
	}

	assert.Equal(t, 2, m.check(CheckQueued))
	assert.Equal(t, 3, m.check(CheckDropped))
	assert.Len(t, w.queue, 2)
}

func TestScan_DroppedTripCanBeQueuedOnNextScan(t *testing.T) {
	trips := staleTrips(2)
	cfg := Config{StaleAfter: time.Minute, Workers: 1, QueueSize: 1, RequestTimeout: time.Second}
	p := &fakePusher{}
	m := newFakeMetrics()
	w, _ := newWorker(t, cfg, newFakeRepo(trips...), p, m)

	w.scan(context.Background())
	require.Equal(t, 1, m.check(CheckDropped))

	w.Start(context.Background())
	stopWithTimeout(t, w)
	require.Equal(t, 1, p.callCount())

	w.mu.Lock()
	assert.Empty(t, w.pending)
	w.mu.Unlock()
}

func TestPool_ProcessesTasksWithFixedNumberOfWorkers(t *testing.T) {
	const workers, tasks = 3, 9

	var inflight, maxInflight atomic.Int64
	entered := make(chan struct{}, tasks)
	release := make(chan struct{})

	p := &fakePusher{fn: func(context.Context, uuid.UUID, uuid.UUID) error {
		cur := inflight.Add(1)
		for {
			prev := maxInflight.Load()
			if cur <= prev || maxInflight.CompareAndSwap(prev, cur) {
				break
			}
		}
		entered <- struct{}{}
		<-release
		inflight.Add(-1)
		return nil
	}}

	cfg := Config{StaleAfter: time.Minute, Workers: workers, QueueSize: tasks, RequestTimeout: 5 * time.Second}
	m := newFakeMetrics()
	w, _ := newWorker(t, cfg, newFakeRepo(staleTrips(tasks)...), p, m)

	w.scan(context.Background())
	w.Start(context.Background())

	for i := 0; i < workers; i++ {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("workers did not pick up tasks")
		}
	}
	close(release)
	stopWithTimeout(t, w)

	assert.Equal(t, tasks, p.callCount())
	assert.Equal(t, int64(workers), maxInflight.Load(), "одновременно работает ровно POSITION_WORKERS воркеров")
	assert.Equal(t, tasks, m.request(RequestSuccess))
}

func TestStartStop_NoGoroutineLeaks(t *testing.T) {
	repo := newFakeRepo(staleTrips(3)...)
	p := &fakePusher{}
	w, ticker := newWorker(t, defaultCfg(), repo, p, newFakeMetrics())

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)

	ticker.tick(t)
	select {
	case <-repo.called:
	case <-time.After(5 * time.Second):
		t.Fatal("scanner did not react to a tick")
	}

	cancel()
	stopWithTimeout(t, w)
}

func TestStop_WithoutStart(t *testing.T) {
	w, _ := newWorker(t, defaultCfg(), newFakeRepo(), &fakePusher{}, newFakeMetrics())
	stopWithTimeout(t, w)
}

func TestProcess_PushErrorDoesNotKillWorker(t *testing.T) {
	trips := staleTrips(2)
	var n atomic.Int64
	p := &fakePusher{fn: func(context.Context, uuid.UUID, uuid.UUID) error {
		if n.Add(1) == 1 {
			return errors.New("push service is down")
		}
		return nil
	}}
	m := newFakeMetrics()
	cfg := Config{StaleAfter: time.Minute, Workers: 1, QueueSize: 10, RequestTimeout: time.Second}
	w, _ := newWorker(t, cfg, newFakeRepo(trips...), p, m)

	w.scan(context.Background())
	w.Start(context.Background())
	stopWithTimeout(t, w)

	assert.Equal(t, 2, p.callCount())
	assert.Equal(t, 1, m.request(RequestError))
	assert.Equal(t, 1, m.request(RequestSuccess))
}

func TestProcess_RequestHasTimeout(t *testing.T) {
	var hasDeadline atomic.Bool
	p := &fakePusher{fn: func(ctx context.Context, _, _ uuid.UUID) error {
		_, ok := ctx.Deadline()
		hasDeadline.Store(ok)
		return nil
	}}
	w, _ := newWorker(t, defaultCfg(), newFakeRepo(staleTrips(1)...), p, newFakeMetrics())

	w.scan(context.Background())
	w.Start(context.Background())
	stopWithTimeout(t, w)

	assert.True(t, hasDeadline.Load())
}

func TestStop_ShutdownTimeoutCancelsInFlightRequests(t *testing.T) {
	entered := make(chan struct{})
	p := &fakePusher{fn: func(ctx context.Context, _, _ uuid.UUID) error {
		close(entered)
		<-ctx.Done()
		return ctx.Err()
	}}
	cfg := Config{StaleAfter: time.Minute, Workers: 1, QueueSize: 5, RequestTimeout: time.Hour}
	w, _ := newWorker(t, cfg, newFakeRepo(staleTrips(1)...), p, newFakeMetrics())

	w.scan(context.Background())
	w.Start(context.Background())
	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := w.Stop(ctx)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestNew_InvalidConfig(t *testing.T) {
	_, err := New(Config{}, newFakeRepo(), &fakePusher{}, newFakeMetrics(), newFakeTicker(), slog.Default())
	require.Error(t, err)
}
