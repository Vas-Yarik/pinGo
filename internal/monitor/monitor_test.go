package monitor

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"pinGo/internal/checker"
	"pinGo/internal/config"
	"pinGo/internal/notify"
	"pinGo/internal/store"
)

type fakePinger struct {
	results []checker.Result
}

func (f *fakePinger) PingAll(context.Context, []config.Target) []checker.Result {
	return f.results
}

type recordingStore struct {
	records []store.Record
	err     error
}

func (s *recordingStore) SaveResult(_ context.Context, r store.Record) error {
	if s.err != nil {
		return s.err
	}
	s.records = append(s.records, r)
	return nil
}

func (s *recordingStore) Close() error { return nil }

type recordingNotifier struct {
	events []notify.Event
	err    error
}

func (n *recordingNotifier) Notify(_ context.Context, e notify.Event) error {
	if n.err != nil {
		return n.err
	}
	n.events = append(n.events, e)
	return nil
}

func testTargets(n int) []config.Target {
	targets := make([]config.Target, n)
	for i := range targets {
		targets[i] = config.Target{Name: fmt.Sprintf("t%d", i), URL: "https://example.com"}
	}
	return targets
}

func res(name string, up bool, status int, latency time.Duration, err error) checker.Result {
	return checker.Result{Target: config.Target{Name: name}, Up: up, Status: status, Latency: latency, Err: err}
}

func newTestMonitor(p Pinger, st store.Storage, n notify.Notifier) *Monitor {
	return New(p, testTargets(2), time.Second, st, n)
}

func upResults() []checker.Result {
	return []checker.Result{
		res("t0", true, 200, 100*time.Millisecond, nil),
		res("t1", true, 200, 200*time.Millisecond, nil),
	}
}

func downResults() []checker.Result {
	return []checker.Result{
		res("t0", true, 200, 100*time.Millisecond, nil),
		res("t1", false, 0, 2*time.Second, errors.New("connection refused")),
	}
}

func TestPollOnceBaselineNoEvents(t *testing.T) {
	st := &recordingStore{}
	n := &recordingNotifier{}
	m := newTestMonitor(&fakePinger{results: downResults()}, st, n)

	m.PollOnce(context.Background())

	if len(st.records) != 2 {
		t.Fatalf("want 2 saved records, got %d", len(st.records))
	}
	if len(n.events) != 0 {
		t.Fatalf("first poll must not emit events, got %v", n.events)
	}
}

func TestPollOnceTransitions(t *testing.T) {
	st := &recordingStore{}
	n := &recordingNotifier{}
	p := &fakePinger{results: upResults()}
	m := newTestMonitor(p, st, n)

	m.PollOnce(context.Background())
	if len(n.events) != 0 {
		t.Fatalf("baseline poll emitted events: %v", n.events)
	}

	p.results = downResults()
	m.PollOnce(context.Background())
	if len(n.events) != 1 {
		t.Fatalf("want 1 event after t1 went down, got %v", n.events)
	}
	down := n.events[0]
	if down.Target != "t1" || down.Transition != notify.TransitionDown || down.Detail != "connection refused" {
		t.Errorf("unexpected down event: %+v", down)
	}

	p.results = upResults()
	m.PollOnce(context.Background())
	if len(n.events) != 2 {
		t.Fatalf("want 2 events total, got %v", n.events)
	}
	up := n.events[1]
	if up.Target != "t1" || up.Transition != notify.TransitionRecovered {
		t.Errorf("unexpected recovered event: %+v", up)
	}
}

func TestPollOnceStoreErrorDoesNotBlockNotify(t *testing.T) {
	st := &recordingStore{err: errors.New("db down")}
	n := &recordingNotifier{}
	m := newTestMonitor(&fakePinger{results: upResults()}, st, n)
	m.PollOnce(context.Background())

	m.pinger = &fakePinger{results: downResults()}
	m.PollOnce(context.Background())

	if len(n.events) != 1 {
		t.Fatalf("notify must run despite store error, got %v", n.events)
	}
}

func TestSnapshotIsACopy(t *testing.T) {
	st := &recordingStore{}
	n := &recordingNotifier{}
	m := newTestMonitor(&fakePinger{results: upResults()}, st, n)
	m.PollOnce(context.Background())

	snap := m.Snapshot()
	if snap.Poll != 1 || len(snap.Targets) != 2 {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	first := snap.Targets[0]
	if !first.Up || first.Polls != 1 || first.Ups != 1 || first.Latency != 100*time.Millisecond {
		t.Errorf("unexpected first state: %+v", first)
	}
	if len(first.Latencies) != 1 {
		t.Fatalf("want 1 latency sample, got %v", first.Latencies)
	}

	snap.Targets[0].Name = "mutated"
	snap.Targets[0].Latencies[0] = time.Hour
	if got := m.Snapshot().Targets[0]; got.Name != "t0" || got.Latencies[0] != 100*time.Millisecond {
		t.Errorf("snapshot mutation leaked into monitor: %+v", got)
	}
}

func TestRingTrimsToCapacity(t *testing.T) {
	m := newTestMonitor(&fakePinger{}, &recordingStore{}, &recordingNotifier{})
	m.ring = 3
	for i := 1; i <= 5; i++ {
		m.pinger = &fakePinger{results: []checker.Result{
			res("t0", true, 200, time.Duration(i)*10*time.Millisecond, nil),
			res("t1", true, 200, time.Duration(i)*10*time.Millisecond, nil),
		}}
		m.PollOnce(context.Background())
	}

	got := m.Snapshot().Targets[0].Latencies
	want := []time.Duration{30 * time.Millisecond, 40 * time.Millisecond, 50 * time.Millisecond}
	if len(got) != len(want) {
		t.Fatalf("want %d samples, got %v", len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sample %d: want %v, got %v", i, want[i], got[i])
		}
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	m := newTestMonitor(&fakePinger{results: upResults()}, &recordingStore{}, &recordingNotifier{})
	var waits int32
	m.wait = func(ctx context.Context, _ time.Duration) error {
		atomic.AddInt32(&waits, 1)
		select {
		case <-time.After(2 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		m.Run(ctx)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancel")
	}
	if atomic.LoadInt32(&waits) < 1 {
		t.Error("wait was never called")
	}
}
