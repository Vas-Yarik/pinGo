package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"pinGo/internal/checker"
	"pinGo/internal/config"
)

const (
	exitOK    = 0
	exitDown  = 1
	exitError = 2
)

const defaultConfigPath = "config.yaml"

func main() {
	os.Exit(run())
}

func run() int {
	configPath := flag.String("config", defaultConfigPath, "path to the YAML configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pingo: %v\n", err)
		return exitError
	}

	results := checker.New().PingAll(context.Background(), cfg.Targets)
	printReport(results)

	for _, res := range results {
		if !res.Up {
			return exitDown
		}
	}
	return exitOK
}

func printReport(results []checker.Result) {
	up := 0
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tHTTP\tLATENCY\tERROR")
	for _, res := range results {
		status := "down"
		code := "-"
		if res.Up {
			up++
			status = "up"
			code = strconv.Itoa(res.Status)
		}
		latency := "-"
		if res.Latency > 0 {
			latency = res.Latency.Round(time.Millisecond).String()
		}
		errMsg := ""
		if res.Err != nil {
			errMsg = res.Err.Error()
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", res.Target.Name, status, code, latency, errMsg)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "pingo: write report: %v\n", err)
		return
	}
	fmt.Printf("%d/%d targets up\n", up, len(results))
}
