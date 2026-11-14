package probe

import (
	"context"
	"net"
	"time"

	"pinGo/internal/config"
)

type TCP struct{}

func (TCP) Kind() string { return "tcp" }

func (TCP) Check(ctx context.Context, t config.Target) Outcome {
	start := time.Now()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", t.Addr)
	latency := time.Since(start)
	if err != nil {
		return Outcome{Latency: latency, Err: err}
	}
	defer conn.Close()

	return Outcome{Latency: latency, Detail: "tcp " + t.Addr}
}
