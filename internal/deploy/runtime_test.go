package deploy_test

import (
	"os"
	"strings"
	"testing"
)

func TestSelfHostComposeUsesLeastPrivilegeRuntime(t *testing.T) {
	data, err := os.ReadFile("../../docker-compose.selfhost.yml")
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}
	compose := string(data)
	for _, want := range []string{
		"read_only: true",
		"cap_drop:\n      - ALL",
		"security_opt:\n      - no-new-privileges:true",
		"/tmp:rw,noexec,nosuid,nodev,size=16m",
	} {
		if !strings.Contains(compose, want) {
			t.Fatalf("compose file missing least-privilege setting %q", want)
		}
	}
}

func TestSelfHostComposeConfiguresLogRetention(t *testing.T) {
	data, err := os.ReadFile("../../docker-compose.selfhost.yml")
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}
	compose := string(data)
	for _, want := range []string{
		"logging:",
		"driver: local",
		`max-size: "10m"`,
		`max-file: "10"`,
	} {
		if !strings.Contains(compose, want) {
			t.Fatalf("compose file missing log retention setting %q", want)
		}
	}
}

func TestSelfHostComposeSetsResourceLimits(t *testing.T) {
	data, err := os.ReadFile("../../docker-compose.selfhost.yml")
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}
	compose := string(data)
	for _, want := range []string{
		`cpus: "1.0"`,
		"mem_limit: 512m",
		"pids_limit: 256",
		"soft: 4096",
		"hard: 4096",
		`cpus: "0.5"`,
		"mem_limit: 256m",
		"pids_limit: 128",
		"soft: 2048",
		"hard: 2048",
	} {
		if !strings.Contains(compose, want) {
			t.Fatalf("compose file missing resource limit %q", want)
		}
	}
}

func TestSelfHostEnvTemplateDocumentsServerLimits(t *testing.T) {
	data, err := os.ReadFile("../../deploy/cashflux-server.env.example")
	if err != nil {
		t.Fatalf("read env template: %v", err)
	}
	env := string(data)
	for _, want := range []string{
		"CASHFLUX_SERVER_HTTP_MAX_IN_FLIGHT=256",
		"CASHFLUX_SERVER_HTTP_RATE_LIMIT_PER_MINUTE=0",
		"CASHFLUX_SERVER_HTTP_USER_RATE_LIMIT_PER_MINUTE=0",
		"CASHFLUX_SERVER_GRPC_MAX_ACTIVE_CONNECTIONS=128",
		"CASHFLUX_SERVER_GRPC_MAX_CONNECTIONS_PER_CLIENT=8",
		"CASHFLUX_SERVER_GRPC_MAX_UPGRADES_PER_CLIENT_PER_MINUTE=60",
		"CASHFLUX_SERVER_GRPC_MAX_STREAMS_PER_USER=8",
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("env template missing server limit %q", want)
		}
	}
}

func TestServerDockerfileRunsAsNonRoot(t *testing.T) {
	data, err := os.ReadFile("../../Dockerfile.server")
	if err != nil {
		t.Fatalf("read server Dockerfile: %v", err)
	}
	dockerfile := string(data)
	for _, want := range []string{
		"adduser -S -G cashflux cashflux",
		"USER cashflux",
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("server Dockerfile missing non-root setting %q", want)
		}
	}
}

func TestObservabilityArtifactsDefineSLOAlerts(t *testing.T) {
	rules, err := os.ReadFile("../../deploy/prometheus-rules.yml")
	if err != nil {
		t.Fatalf("read prometheus rules: %v", err)
	}
	ruleText := string(rules)
	for _, want := range []string{
		"CashFluxBackendDown",
		"CashFluxHighErrorRate",
		"CashFluxHighGRPCErrorRate",
		"CashFluxHighHTTPLatency",
		"cashflux_server_up",
		"cashflux_http_requests_total",
		"cashflux_grpc_requests_total",
		"cashflux_http_request_duration_seconds_bucket",
		"histogram_quantile",
		"severity: page",
	} {
		if !strings.Contains(ruleText, want) {
			t.Fatalf("prometheus rules missing %q", want)
		}
	}

	runbook, err := os.ReadFile("../../docs/OBSERVABILITY.md")
	if err != nil {
		t.Fatalf("read observability runbook: %v", err)
	}
	runbookText := string(runbook)
	for _, want := range []string{"Logs", "local` log driver", "30 days", "Service-Level Objectives", "Dashboard Queries", "Alerts And Routing", "trace_id"} {
		if !strings.Contains(runbookText, want) {
			t.Fatalf("observability runbook missing %q", want)
		}
	}
}

func TestBackupArtifactsDefineScheduleAndRestoreRunbook(t *testing.T) {
	service, err := os.ReadFile("../../deploy/cashflux-backup.example.service")
	if err != nil {
		t.Fatalf("read backup service: %v", err)
	}
	serviceText := string(service)
	for _, want := range []string{
		"cashflux-server backup /data/backups",
		"CASHFLUX_OFFBOX_TARGET",
		"rclone sync",
		"CASHFLUX_BACKUP_DIR",
	} {
		if !strings.Contains(serviceText, want) {
			t.Fatalf("backup service missing %q", want)
		}
	}

	timer, err := os.ReadFile("../../deploy/cashflux-backup.example.timer")
	if err != nil {
		t.Fatalf("read backup timer: %v", err)
	}
	timerText := string(timer)
	for _, want := range []string{"OnCalendar=", "Persistent=true", "RandomizedDelaySec="} {
		if !strings.Contains(timerText, want) {
			t.Fatalf("backup timer missing %q", want)
		}
	}

	runbook, err := os.ReadFile("../../docs/SELF_HOSTING.md")
	if err != nil {
		t.Fatalf("read self-host runbook: %v", err)
	}
	runbookText := string(runbook)
	for _, want := range []string{
		"cashflux-server backup /data/backups",
		"manifest.json",
		"off-box",
		"Restore rehearsal",
		"RPO is the last successful scheduled backup",
		"RTO is the time to restore",
	} {
		if !strings.Contains(runbookText, want) {
			t.Fatalf("self-host runbook missing %q", want)
		}
	}
}
