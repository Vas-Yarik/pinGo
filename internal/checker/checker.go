package checker

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"pinGo/internal/config"
)

const userAgent = "pingo/0.1 (+https://github.com/Vas-Yarik/pinGo)"

type Result struct {
	Target  config.Target
	Up      bool
	Status  int
	Latency time.Duration
	Err     error
}

type Checker struct {
	client *http.Client
}

func New() *Checker {
	return &Checker{client: &http.Client{}}
}

func (c *Checker) Ping(ctx context.Context, t config.Target) Result {
	ctx, cancel := context.WithTimeout(ctx, t.ResolvedTimeout())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		return Result{Target: t, Err: fmt.Errorf("build request: %w", err)}
	}
	req.Header.Set("User-Agent", userAgent)

	start := time.Now()
	resp, err := c.client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return Result{Target: t, Latency: latency, Err: err}
	}
	defer resp.Body.Close()

	return Result{Target: t, Up: true, Status: resp.StatusCode, Latency: latency}
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
