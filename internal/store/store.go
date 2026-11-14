package store

import (
	"context"
	"time"
)

type Record struct {
	Target    string
	Up        bool
	LatencyMS int64
	Detail    string
	Err       string
	At        time.Time
}

type Storage interface {
	SaveResult(ctx context.Context, r Record) error
	Close() error
}

type Noop struct{}

func (Noop) SaveResult(context.Context, Record) error { return nil }

func (Noop) Close() error { return nil }
