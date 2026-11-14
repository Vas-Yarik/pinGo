package checker

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pinGo/internal/config"
)

func target(url string, timeout time.Duration) config.Target {
	return config.Target{Name: "test", URL: url, Timeout: timeout}
}

func TestPingUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); !strings.HasPrefix(got, "pingo/") {
			t.Errorf("want pingo user agent, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := New().Ping(context.Background(), target(srv.URL, time.Second))
	if !res.Up {
		t.Fatalf("want up, got down: %v", res.Err)
	}
	if res.Status != http.StatusOK {
		t.Errorf("want status 200, got %d", res.Status)
	}
	if res.Latency < 0 {
		t.Errorf("negative latency: %v", res.Latency)
	}
}

func TestPingErrorResponseIsUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	res := New().Ping(context.Background(), target(srv.URL, time.Second))
	if !res.Up {
		t.Fatalf("want up for a 404 response, got down: %v", res.Err)
	}
	if res.Status != http.StatusNotFound {
		t.Errorf("want status 404, got %d", res.Status)
	}
}

func TestPingTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer srv.Close()

	res := New().Ping(context.Background(), target(srv.URL, 50*time.Millisecond))
	if res.Up {
		t.Fatal("want down for a timed-out request")
	}
	if res.Err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestPingConnectionRefused(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	res := New().Ping(context.Background(), target("http://"+addr, 2*time.Second))
	if res.Up {
		t.Fatal("want down for a closed port")
	}
	if res.Err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestPingAllRunsConcurrentlyAndKeepsOrder(t *testing.T) {
	const n = 4
	srvs := make([]*httptest.Server, n)
	targets := make([]config.Target, n)
	for i := range srvs {
		srvs[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusTeapot)
		}))
		defer srvs[i].Close()
		targets[i] = target(srvs[i].URL, time.Second)
	}

	start := time.Now()
	results := New().PingAll(context.Background(), targets)
	elapsed := time.Since(start)

	if elapsed >= 300*time.Millisecond {
		t.Errorf("checks do not run concurrently: took %v", elapsed)
	}
	for i, res := range results {
		if !res.Up {
			t.Errorf("target %d: want up, got %v", i, res.Err)
		}
		if res.Status != http.StatusTeapot {
			t.Errorf("target %d: want status 418, got %d", i, res.Status)
		}
		if res.Target.URL != srvs[i].URL {
			t.Errorf("target %d: result order does not match input order", i)
		}
	}
}
