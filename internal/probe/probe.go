package probe

import (
	"context"
	"fmt"
	"time"

	"pinGo/internal/config"
)

type Outcome struct {
	Latency time.Duration
	Detail  string
	Err     error
}

type Probe interface {
	Kind() string
	Check(ctx context.Context, t config.Target) Outcome
}

func For(kind string) (Probe, error) {
	switch kind {
	case "", "http":
		return HTTP{}, nil
	case "tcp":
		return TCP{}, nil
	}
	return nil, fmt.Errorf("unknown probe type %q", kind)
}
