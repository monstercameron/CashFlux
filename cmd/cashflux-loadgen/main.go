// SPDX-License-Identifier: MIT

// Command cashflux-loadgen benchmarks and stress-tests a live CashFlux
// backend over the real client protocol (gRPC-over-WS sync + HTTP blobs).
//
// Typical runs:
//
//	# Baseline capacity: 200 quietly-active clients for 5 minutes.
//	cashflux-loadgen -server http://127.0.0.1:8790 -token dev-token \
//	    -scenario steady -clients 200 -duration 5m
//
//	# Restart thundering herd.
//	cashflux-loadgen -server ... -scenario stampede -clients 500 -duration 2m
//
//	# Two-device conflict churn (clients paired onto shared workspaces).
//	cashflux-loadgen -server ... -scenario conflict -clients 50 -duration 1m
//
// The report prints p50/p95/p99/max latency and ops/sec per operation, byte
// volumes, and counters (push_accepted / push_rejected / watch_events / …).
// Identical flags + -seed reproduce the identical schedule, so runs are
// comparable across hardware (e.g. Stack A vs Stack B in the business plan).
// Never point this at a server holding real user data.
//
// Load-test server setup: all virtual clients share one source IP and token,
// so the server's per-client abuse guards (which are a FEATURE in production
// — default 8 concurrent bridge connections per client) will throttle the
// fleet unless raised. Start the throwaway server with:
//
//	CASHFLUX_SERVER_ADDR=127.0.0.1:8796
//	CASHFLUX_SERVER_TOKEN=<dev token>
//	CASHFLUX_SERVER_DATA_DIR=<throwaway dir>
//	CASHFLUX_SERVER_GRPC_MAX_ACTIVE_CONNECTIONS=4096
//	CASHFLUX_SERVER_GRPC_MAX_CONNECTIONS_PER_CLIENT=4096
//	CASHFLUX_SERVER_GRPC_MAX_UPGRADES_PER_CLIENT_PER_MINUTE=100000
//	CASHFLUX_SERVER_GRPC_MAX_STREAMS_PER_USER=4096
//
// To measure the abuse guards themselves, leave the defaults and expect
// throttling to appear as errors past 8 concurrent clients.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/monstercameron/CashFlux/internal/loadgen"
)

func main() {
	var (
		serverURL = flag.String("server", "http://127.0.0.1:8790", "backend base URL")
		token     = flag.String("token", "dev-token", "auth token (token auth mode)")
		scenario  = flag.String("scenario", string(loadgen.ScenarioSteady), "workload: steady|storm|stampede|conflict|blob|mixed")
		clients   = flag.Int("clients", 50, "number of virtual clients")
		duration  = flag.Duration("duration", time.Minute, "run length")
		seed      = flag.Int64("seed", 1, "schedule seed (same flags+seed = identical run)")
		dataset   = flag.Int("dataset-bytes", 0, "override pushed dataset size (0 = scenario default)")
		blob      = flag.Int("blob-bytes", 0, "override blob size (0 = scenario default)")
		watch     = flag.Int("watch-every", 4, "open a watch stream on every Nth client (0 = none)")
		out       = flag.String("out", "", "also write the report as JSON to this file")
		maxErrPct = flag.Float64("max-error-pct", 1.0, "exit non-zero if the error rate exceeds this percentage")
		list      = flag.Bool("scenarios", false, "list scenarios and exit")
	)
	flag.Parse()

	if *list {
		for _, s := range loadgen.Scenarios() {
			fmt.Printf("%-10s %s\n", s, loadgen.Describe(s))
		}
		return
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	profile := loadgen.DefaultProfile(loadgen.Scenario(*scenario))
	if *dataset > 0 {
		profile.DatasetBytes = *dataset
	}
	if *blob > 0 {
		profile.BlobBytes = *blob
	}
	plan, err := loadgen.BuildPlan(loadgen.Scenario(*scenario), *clients, *duration, *seed, profile)
	if err != nil {
		log.Error("invalid run configuration", slog.String("err", err.Error()))
		os.Exit(2)
	}

	// Ctrl-C stops clients at the next op boundary; the partial report still
	// prints so an aborted run is never wasted.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	driver := &loadgen.Driver{ServerURL: *serverURL, Token: *token, WatchEvery: *watch, Logger: log}
	report, err := driver.Run(ctx, plan)
	if err != nil {
		log.Error("run failed", slog.String("err", err.Error()))
		os.Exit(2)
	}

	fmt.Print(report.String())

	if *out != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err == nil {
			err = os.WriteFile(*out, data, 0o600)
		}
		if err != nil {
			log.Error("write report", slog.String("path", *out), slog.String("err", err.Error()))
			os.Exit(2)
		}
		log.Info("report written", slog.String("path", *out))
	}

	if total := report.TotalOps(); total > 0 {
		errPct := 100 * float64(report.TotalErrors()) / float64(total)
		if errPct > *maxErrPct {
			log.Error("error rate over threshold",
				slog.String("errorRate", fmt.Sprintf("%.2f%%", errPct)),
				slog.String("threshold", fmt.Sprintf("%.2f%%", *maxErrPct)))
			os.Exit(1)
		}
	}
}
