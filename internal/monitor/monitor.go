package monitor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"pinGo/internal/checker"
	"pinGo/internal/config"
	"pinGo/internal/notify"
	"pinGo/internal/store"
)

const ringSize = 30

type Pinger interface {
	PingAll(ctx context.Context, targets []config.Target) []checker.Result
}

type Waiter func(ctx context.Context, d time.Duration) error

type TargetState struct {
	Name      string
	Up        bool
	Status    int
	Latency   time.Duration
	Err       string
	CheckedAt time.Time
	Polls     int
	Ups       int
	Latencies []time.Duration
}

type Snapshot struct {
	Targets     []TargetState
	Poll        int
	LastChecked time.Time
	Interval    time.Duration
	LastEvent   *notify.Event
}

type targetState struct {
	name      string
	checked   bool
	up        bool
	status    int
	latency   time.Duration
	err       string
	checkedAt time.Time
	polls     int
	ups       int
	latencies []time.Duration
}

type Monitor struct {
	pinger   Pinger
	targets  []config.Target
	interval time.Duration
	ring     int
	save     store.Storage
	alert    notify.Notifier
	wait     Waiter

	mu          sync.RWMutex
	poll        int
	lastChecked time.Time
	lastEvent   *notify.Event
	states      map[string]*targetState
	order       []string
}

func New(pinger Pinger, targets []config.Target, interval time.Duration, save store.Storage, alert notify.Notifier) *Monitor {
	states := make(map[string]*targetState, len(targets))
	order := make([]string, 0, len(targets))
	for _, t := range targets {
		states[t.Name] = &targetState{name: t.Name}
		order = append(order, t.Name)
	}
	return &Monitor{
		pinger:   pinger,
		targets:  targets,
		interval: interval,
		ring:     ringSize,
		save:     save,
		alert:    alert,
		wait:     waitDefault,
		states:   states,
		order:    order,
	}
}

func (m *Monitor) Run(ctx context.Context) {
	for {
		m.PollOnce(ctx)
		if err := m.wait(ctx, m.interval); err != nil {
			return
		}
	}
}

func (m *Monitor) PollOnce(ctx context.Context) {
	results := m.pinger.PingAll(ctx, m.targets)
	events := m.record(results)

	for _, r := range results {
		rec := store.Record{
			Target:    r.Target.Name,
			Up:        r.Up,
			Status:    r.Status,
			LatencyMS: r.Latency.Milliseconds(),
			At:        time.Now(),
		}
		if r.Err != nil {
			rec.Err = r.Err.Error()
		}
		if err := m.save.SaveResult(ctx, rec); err != nil {
			log.Printf("pingo: save result for %s: %v", r.Target.Name, err)
		}
	}
	for _, e := range events {
		if err := m.alert.Notify(ctx, e); err != nil {
			log.Printf("pingo: notify %s %s: %v", e.Target, e.Transition, err)
		}
	}
}

func (m *Monitor) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := Snapshot{
		Poll:        m.poll,
		LastChecked: m.lastChecked,
		Interval:    m.interval,
	}
	if m.lastEvent != nil {
		e := *m.lastEvent
		out.LastEvent = &e
	}
	out.Targets = make([]TargetState, 0, len(m.order))
	for _, name := range m.order {
		s := m.states[name]
		ts := TargetState{
			Name:      s.name,
			Up:        s.up,
			Status:    s.status,
			Latency:   s.latency,
			Err:       s.err,
			CheckedAt: s.checkedAt,
			Polls:     s.polls,
			Ups:       s.ups,
		}
		if len(s.latencies) > 0 {
			ts.Latencies = append([]time.Duration(nil), s.latencies...)
		}
		out.Targets = append(out.Targets, ts)
	}
	return out
}

func (m *Monitor) record(results []checker.Result) []notify.Event {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	m.poll++
	m.lastChecked = now
	events := make([]notify.Event, 0, len(results))
	for _, r := range results {
		s := m.states[r.Target.Name]
		var e *notify.Event
		if s.checked {
			switch {
			case s.up && !r.Up:
				e = &notify.Event{Target: r.Target.Name, Transition: notify.TransitionDown, Detail: r.Err.Error(), At: now}
			case !s.up && r.Up:
				e = &notify.Event{Target: r.Target.Name, Transition: notify.TransitionRecovered, Detail: fmt.Sprintf("status %d, latency %s", r.Status, r.Latency.Round(time.Millisecond)), At: now}
			}
		}

		s.checked = true
		s.up = r.Up
		s.status = r.Status
		s.latency = r.Latency
		s.checkedAt = now
		s.polls++
		if r.Err != nil {
			s.err = r.Err.Error()
		} else {
			s.err = ""
		}
		if r.Up {
			s.ups++
			if r.Latency > 0 {
				s.latencies = append(s.latencies, r.Latency)
				if len(s.latencies) > m.ring {
					s.latencies = s.latencies[len(s.latencies)-m.ring:]
				}
			}
		}
		if e != nil {
			m.lastEvent = e
			events = append(events, *e)
		}
	}
	return events
}

func waitDefault(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
