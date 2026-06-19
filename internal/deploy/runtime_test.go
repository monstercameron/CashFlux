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
		"CASHFLUX_SERVER_STORAGE_MAX_BYTES=0",
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

func TestSelfHostEnvTemplateDoesNotShipDefaultMasterKey(t *testing.T) {
	data, err := os.ReadFile("../../deploy/cashflux-server.env.example")
	if err != nil {
		t.Fatalf("read env template: %v", err)
	}
	env := string(data)
	if strings.Contains(env, "0123456789abcdef0123456789abcdef") {
		t.Fatal("env template ships a default-looking master key")
	}
	if !strings.Contains(env, "CASHFLUX_SERVER_MASTER_KEY=replace-with-32-byte-secret-from-secret-manager") {
		t.Fatal("env template does not direct operators to provide a secret-managed master key")
	}
}

func TestSelfHostDocsDefineMasterKeyHandling(t *testing.T) {
	data, err := os.ReadFile("../../docs/SELF_HOSTING.md")
	if err != nil {
		t.Fatalf("read self-host docs: %v", err)
	}
	doc := string(data)
	for _, want := range []string{
		"CASHFLUX_SERVER_MASTER_KEY",
		"secret manager",
		"KMS-backed secret",
		"exactly 16, 24, or 32 bytes",
		"AES-GCM",
		"re-encrypted under the new key",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("self-host docs missing master-key guidance %q", want)
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

func TestServerReleaseHelperDefinesSupplyChainArtifacts(t *testing.T) {
	data, err := os.ReadFile("../../deploy/release-server.example.sh")
	if err != nil {
		t.Fatalf("read release helper: %v", err)
	}
	helper := string(data)
	for _, want := range []string{
		"CGO_ENABLED=0 go build",
		"-trimpath",
		"-buildvcs=true",
		"-buildid=",
		"sha256sum",
		"cyclonedx-gomod",
		"cosign sign-blob",
		".cdx.json",
		".sig",
	} {
		if !strings.Contains(helper, want) {
			t.Fatalf("release helper missing %q", want)
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

func TestBlobGCArtifactsDefineSchedule(t *testing.T) {
	service, err := os.ReadFile("../../deploy/cashflux-blob-gc.example.service")
	if err != nil {
		t.Fatalf("read blob gc service: %v", err)
	}
	if !strings.Contains(string(service), "cashflux-server gc-blobs") {
		t.Fatalf("blob gc service missing command: %s", service)
	}
	timer, err := os.ReadFile("../../deploy/cashflux-blob-gc.example.timer")
	if err != nil {
		t.Fatalf("read blob gc timer: %v", err)
	}
	timerText := string(timer)
	for _, want := range []string{"OnCalendar=", "Persistent=true", "RandomizedDelaySec="} {
		if !strings.Contains(timerText, want) {
			t.Fatalf("blob gc timer missing %q", want)
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

func TestBackendSecurityNotesDocumentProtectedRoutes(t *testing.T) {
	data, err := os.ReadFile("../../docs/BACKEND_SECURITY.md")
	if err != nil {
		t.Fatalf("read backend security notes: %v", err)
	}
	doc := string(data)
	for _, want := range []string{
		"deny-by-default",
		"/v1/audit",
		"/v1/blobs/{hash}",
		"cashflux.v1.SyncService",
		"cashflux.v1.AIService",
		"auth interceptors",
		"Security Coverage Map",
		"AI keys are encrypted at rest with AES-GCM",
		"Strict tenant isolation",
		"Request-size and abuse controls",
		"Load/abuse tests cover oversized sync snapshots",
		"per-user workspace stream caps",
		"gRPC bridge connection limit",
		"Unit tests cover server storage",
		"AES-GCM AI-key encrypt/decrypt/rotation",
		"content-addressed blob hashing",
		"Gitleaks",
		"govulncheck",
		"gosec",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("backend security notes missing %q", want)
		}
	}
}

func TestReportExportDesignDocumentsOfflineSnapshots(t *testing.T) {
	data, err := os.ReadFile("../../docs/REPORT_EXPORTS.md")
	if err != nil {
		t.Fatalf("read report export design: %v", err)
	}
	doc := string(data)
	for _, want := range []string{
		"snapshot the already-rendered static SVG markup",
		"must not depend on live D3",
		"PDF",
		"standalone HTML",
		"PNG",
		"CSV",
		"JSON",
		"contains financial data",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("report export design missing %q", want)
		}
	}

	sw, err := os.ReadFile("../../web/sw.js")
	if err != nil {
		t.Fatalf("read service worker: %v", err)
	}
	if !strings.Contains(string(sw), "https://cdn.jsdelivr.net/npm/d3@7.9.0/dist/d3.min.js") {
		t.Fatal("service worker does not pin/cache D3 7.9.0")
	}
}

func TestInvestmentsScopeDocumentsBalanceOnlyDecision(t *testing.T) {
	data, err := os.ReadFile("../../docs/INVESTMENTS_SCOPE.md")
	if err != nil {
		t.Fatalf("read investments scope doc: %v", err)
	}
	doc := string(data)
	for _, want := range []string{
		"balance-only",
		"Do not add holdings",
		"live market pricing",
		"local-first",
		"manual extension",
		"symbol, quantity, manual price, and as-of date",
		"Net-worth reports may continue to use account balances",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("investments scope doc missing %q", want)
		}
	}
}

func TestBackendToolchainPinnedForServerAndWASM(t *testing.T) {
	goMod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !strings.Contains(string(goMod), "\ngo 1.26.0\n") {
		t.Fatal("go.mod does not pin Go 1.26.0")
	}

	dockerfile, err := os.ReadFile("../../Dockerfile.server")
	if err != nil {
		t.Fatalf("read Dockerfile.server: %v", err)
	}
	for _, want := range []string{
		"FROM golang:1.26-alpine AS build",
		"go build -trimpath -ldflags=\"-s -w\" -o /out/cashflux-server ./cmd/cashflux-server",
	} {
		if !strings.Contains(string(dockerfile), want) {
			t.Fatalf("Dockerfile.server missing %q", want)
		}
	}
}

func TestBackendPlanDocumentsAIOverGRPC(t *testing.T) {
	data, err := os.ReadFile("../../docs/BACKEND_PLAN.md")
	if err != nil {
		t.Fatalf("read backend plan: %v", err)
	}
	doc := string(data)
	for _, want := range []string{
		"AIService.SetKey",
		"AIService.Chat",
		"AIService.Vision",
		"GoGRPCBridge `/grpc` tunnel",
		"The legacy HTTP AI routes are retired",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("backend plan missing %q", want)
		}
	}
	for _, legacy := range []string{
		"POST /v1/ai/key",
		"POST /v1/ai/chat",
		"POST /v1/ai/vision",
		"streams SSE",
		"SSE streaming",
	} {
		if strings.Contains(doc, legacy) {
			t.Fatalf("backend plan still documents legacy AI transport %q", legacy)
		}
	}
}

func TestBackendPlanDocumentsPhasedRollout(t *testing.T) {
	data, err := os.ReadFile("../../docs/BACKEND_PLAN.md")
	if err != nil {
		t.Fatalf("read backend plan: %v", err)
	}
	doc := string(data)
	for _, want := range []string{
		"## Phasing (each independently shippable)",
		"Auth + snapshot sync (LWW)",
		"artifacts still inline",
		"Blob store + client artifact extraction",
		"AI proxy + encrypted keys + metering",
		"Rollout rule: each phase must be independently shippable and reversible",
		"The local-first app keeps working at",
		"every phase",
		"fall",
		"back to the prior phase",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("backend plan missing rollout text %q", want)
		}
	}
}

func TestSelfHostCaddyKeepsGRPCWebsocketStreamsAlive(t *testing.T) {
	caddy, err := os.ReadFile("../../deploy/Caddyfile.selfhost")
	if err != nil {
		t.Fatalf("read self-host Caddyfile: %v", err)
	}
	caddyfile := string(caddy)
	for _, want := range []string{
		"{$CASHFLUX_DOMAIN}",
		"reverse_proxy cashflux-server:8081",
		"header_up X-Forwarded-Proto {scheme}",
		"header_up X-Forwarded-Host {host}",
		"keepalive 2m",
		"keepalive_interval 30s",
		"stream_timeout 24h",
		"stream_close_delay 5m",
	} {
		if !strings.Contains(caddyfile, want) {
			t.Fatalf("self-host Caddyfile missing %q", want)
		}
	}

	doc, err := os.ReadFile("../../docs/SELF_HOSTING.md")
	if err != nil {
		t.Fatalf("read self-hosting doc: %v", err)
	}
	text := string(doc)
	for _, want := range []string{
		"wss://<domain>/grpc",
		"long-lived `/grpc` websocket streams",
		"avoid short idle",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("self-hosting doc missing %q", want)
		}
	}
}

func TestSelfHostingDocumentsSingleBinaryDataDirAndBackups(t *testing.T) {
	doc, err := os.ReadFile("../../docs/SELF_HOSTING.md")
	if err != nil {
		t.Fatalf("read self-hosting doc: %v", err)
	}
	text := string(doc)
	for _, want := range []string{
		"one `cashflux-server` binary",
		"CASHFLUX_SERVER_DATA_DIR",
		"cashflux-data",
		"cashflux-server backup",
		"checkpoints SQLite WAL",
		"copies `cashflux-server.db` and `blobs/`",
		"RPO is the last successful scheduled backup",
		"Migrations run on server startup",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("self-hosting doc missing %q", want)
		}
	}

	dockerfile, err := os.ReadFile("../../Dockerfile.server")
	if err != nil {
		t.Fatalf("read Dockerfile.server: %v", err)
	}
	if !strings.Contains(string(dockerfile), "ENTRYPOINT [\"cashflux-server\"]") {
		t.Fatal("server Dockerfile does not ship the cashflux-server entrypoint")
	}
	compose, err := os.ReadFile("../../docker-compose.selfhost.yml")
	if err != nil {
		t.Fatalf("read self-host compose: %v", err)
	}
	for _, want := range []string{
		"env_file:",
		"cashflux-data:/data",
		"caddy:",
	} {
		if !strings.Contains(string(compose), want) {
			t.Fatalf("self-host compose missing %q", want)
		}
	}
}

func TestCIIncludesServerBuildAndSecurityScans(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatalf("read ci workflow: %v", err)
	}
	workflow := string(data)
	for _, want := range []string{
		"go vet ./...",
		"govulncheck",
		"gosec",
		"gitleaks",
		"go test ./...",
		"go build ./cmd/cashflux-server",
		"GOOS: js",
		"GOARCH: wasm",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("ci workflow missing %q", want)
		}
	}
}
