package probe

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"pinGo/internal/config"
)

const userAgent = "pingo/0.1 (+https://github.com/Vas-Yarik/pinGo)"

type HTTP struct{}

func (HTTP) Kind() string { return "http" }

func (HTTP) Check(ctx context.Context, t config.Target) Outcome {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		return Outcome{Err: fmt.Errorf("build request: %w", err)}
	}
	req.Header.Set("User-Agent", userAgent)

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		return Outcome{Latency: latency, Err: err}
	}
	defer resp.Body.Close()

	return Outcome{Latency: latency, Detail: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}
