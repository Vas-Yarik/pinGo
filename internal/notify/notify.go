package notify

import (
	"context"
	"time"
)

const (
	TransitionDown      = "down"
	TransitionRecovered = "recovered"
)

type Event struct {
	Target     string
	Transition string
	Detail     string
	At         time.Time
}

type Notifier interface {
	Notify(ctx context.Context, e Event) error
}

type Noop struct{}

func (Noop) Notify(context.Context, Event) error { return nil }
