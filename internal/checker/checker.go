package checker

import (
	"context"
	"sync"
	"time"

	"pinGo/internal/config"
	"pinGo/internal/probe"
)

type Result struct {
	Target  config.Target
	Up      bool
	Latency time.Duration
	Detail  string
	Err     error
}

type Checker struct{}

func New() *Checker {
	return &Checker{}
}

func (c *Checker) Ping(ctx context.Context, t config.Target) Result {
	ctx, cancel := context.WithTimeout(ctx, t.ResolvedTimeout())
	defer cancel()

	p, err := probe.For(t.Type)
	if err != nil {
		return Result{Target: t, Err: err}
	}
	out := p.Check(ctx, t)
	return Result{
		Target:  t,
		Up:      out.Err == nil,
		Latency: out.Latency,
		Detail:  out.Detail,
		Err:     out.Err,
	}
}

func (c *Checker) PingAll(ctx context.Context, targets []config.Target) []Result {
	results := make([]Result, len(targets))
	var wg sync.WaitGroup
	wg.Add(len(targets))
	for i := range targets {
		t := targets[i]
		go func() {
			defer wg.Done()
			results[i] = c.Ping(ctx, t)
		}()
	}
	wg.Wait()
	return results
}
