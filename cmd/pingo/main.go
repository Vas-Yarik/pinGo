package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	isatty "github.com/mattn/go-isatty"

	"pinGo/internal/asset"
	"pinGo/internal/checker"
	"pinGo/internal/config"
	"pinGo/internal/monitor"
	"pinGo/internal/notify"
	"pinGo/internal/store"
	"pinGo/internal/tui"
)

const (
	exitOK    = 0
	exitDown  = 1
	exitError = 2
)

const defaultConfigPath = "config.yaml"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	mode := "check"
	rest := args
	if len(args) > 0 {
		switch args[0] {
		case "check", "watch", "daemon":
			mode = args[0]
			rest = args[1:]
		}
	}

	fs := flag.NewFlagSet(mode, flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath, "path to the YAML configuration file")
	if err := fs.Parse(rest); err != nil {
		return exitError
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pingo: %v\n", err)
		return exitError
	}

	switch mode {
	case "watch":
		return runWatch(cfg)
	case "daemon":
		return runDaemon(cfg)
	default:
		return runCheck(cfg)
	}
}

func runCheck(cfg *config.Config) int {
	results := checker.New().PingAll(context.Background(), cfg.Targets)
	printReport(results)
	for _, r := range results {
		if !r.Up {
			return exitDown
		}
	}
	return exitOK
}

func runDaemon(cfg *config.Config) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mon := monitor.New(checker.New(), cfg.Targets, cfg.Interval, store.Noop{}, newNotifier())
	fmt.Fprintf(os.Stderr, "pingo: daemon started, %d targets, interval %s\n", len(cfg.Targets), cfg.Interval)
	mon.Run(ctx)
	fmt.Fprintln(os.Stderr, "pingo: daemon stopped")
	return exitOK
}

func runWatch(cfg *config.Config) int {
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(os.Stderr, "pingo: watch mode requires an interactive terminal")
		return exitError
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mon := monitor.New(checker.New(), cfg.Targets, cfg.Interval, store.Noop{}, newNotifier())
	go mon.Run(ctx)

	if err := tui.Run(mon); err != nil {
		fmt.Fprintf(os.Stderr, "pingo: tui: %v\n", err)
		return exitError
	}
	return exitOK
}

func newNotifier() notify.Notifier {
	d, err := notify.NewDesktop(asset.PinGoIconUp, asset.PinGoIconDown)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pingo: os notifications disabled: %v\n", err)
		return notify.Noop{}
	}
	return d
}

func printReport(results []checker.Result) {
	up := 0
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATE\tDETAIL\tLATENCY\tERROR")
	for _, res := range results {
		state := "down"
		if res.Up {
			up++
			state = "up"
		}
		latency := "-"
		if res.Latency > 0 {
			latency = res.Latency.Round(time.Millisecond).String()
		}
		errMsg := ""
		if res.Err != nil {
			errMsg = res.Err.Error()
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", res.Target.Name, state, res.Detail, latency, errMsg)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "pingo: write report: %v\n", err)
		return
	}
	fmt.Printf("%d/%d targets up\n", up, len(results))
}
