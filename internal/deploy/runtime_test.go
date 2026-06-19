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
		"CASHFLUX_SERVER_AUDIT_RETENTION_DAYS=365",
		"CASHFLUX_SERVER_SNAPSHOT_HISTORY_RETENTION_DAYS=180",
		"CASHFLUX_SERVER_BACKUP_RETENTION_DAYS=30",
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

func TestRetentionArtifactsDefineSchedule(t *testing.T) {
	service, err := os.ReadFile("../../deploy/cashflux-retention.example.service")
	if err != nil {
		t.Fatalf("read retention service: %v", err)
	}
	if !strings.Contains(string(service), "cashflux-server retention") {
		t.Fatalf("retention service missing command: %s", service)
	}
	timer, err := os.ReadFile("../../deploy/cashflux-retention.example.timer")
	if err != nil {
		t.Fatalf("read retention timer: %v", err)
	}
	timerText := string(timer)
	for _, want := range []string{"OnCalendar=", "Persistent=true", "RandomizedDelaySec="} {
		if !strings.Contains(timerText, want) {
			t.Fatalf("retention timer missing %q", want)
		}
	}
}

func TestIncidentResponseRunbookDefinesStatusAndComms(t *testing.T) {
	data, err := os.ReadFile("../../docs/INCIDENT_RESPONSE.md")
	if err != nil {
		t.Fatalf("read incident runbook: %v", err)
	}
	runbook := string(data)
	for _, want := range []string{
		"GET /status",
		"SEV1",
		"SEV2",
		"SEV3",
		"First 15 Minutes",
		"Communication",
		"Postmortem",
		"docs/SELF_HOSTING.md",
	} {
		if !strings.Contains(runbook, want) {
			t.Fatalf("incident runbook missing %q", want)
		}
	}
}

func TestOperationsRunbookDefinesRequiredProcedures(t *testing.T) {
	data, err := os.ReadFile("../../docs/OPERATIONS_RUNBOOK.md")
	if err != nil {
		t.Fatalf("read operations runbook: %v", err)
	}
	runbook := string(data)
	for _, want := range []string{
		"## Deploy",
		"## Rollback",
		"## Restore",
		"## Rotate Access Token",
		"## Rotate Master Key",
		"## Revoke Sessions",
		"## Past-Due Billing",
		"cashflux-server backup",
		"cashflux-server rotate-token",
		"/v1/audit",
	} {
		if !strings.Contains(runbook, want) {
			t.Fatalf("operations runbook missing %q", want)
		}
	}
}

func TestScaleLimitsDocumentSQLiteCeilingAndMigrationPath(t *testing.T) {
	data, err := os.ReadFile("../../docs/SCALE_LIMITS.md")
	if err != nil {
		t.Fatalf("read scale limits doc: %v", err)
	}
	doc := string(data)
	for _, want := range []string{
		"SQLite allows many readers but still has one writer",
		"Do Not Guess Capacity",
		"Migration Path",
		"Postgres",
		"object storage",
		"Tenant-isolation tests",
		"restore",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("scale limits doc missing %q", want)
		}
	}
}
