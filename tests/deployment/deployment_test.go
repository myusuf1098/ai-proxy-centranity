package deployment_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/health"
)

func TestDeployment_DockerComposeArtifacts(t *testing.T) {
	composeContent, err := os.ReadFile("../../docker-compose.yml")
	if err != nil {
		t.Fatalf("failed to read docker-compose.yml: %v", err)
	}

	content := string(composeContent)
	requiredServices := []string{"proxygateway-api", "postgres", "redis"}
	for _, svc := range requiredServices {
		if !strings.Contains(content, svc) {
			t.Errorf("docker-compose.yml missing expected service: %s", svc)
		}
	}

	if strings.HasPrefix(content, "version:") {
		t.Errorf("docker-compose.yml must not declare a deprecated top-level version")
	}

	// backend network must be internal (postgres/redis have no host ingress)
	if !strings.Contains(content, "backend:") || !strings.Contains(content, "internal: true") {
		t.Errorf("docker-compose.yml must set backend network internal: true")
	}

	// every service must carry a mem_limit (compose non-swarm ignores deploy.resources)
	for _, svc := range []string{"postgres:", "redis:", "proxygateway-api:"} {
		idx := strings.Index(content, svc)
		if idx < 0 {
			t.Errorf("docker-compose.yml missing service block: %s", svc)
			continue
		}
		block := content[idx : idx+400]
		if !strings.Contains(block, "mem_limit:") {
			t.Errorf("service %s missing mem_limit", svc)
		}
	}

	// no hardcoded secret defaults may remain
	for _, secret := range []string{"pg_centranity_secure_2026", "pg_admin_centranity_token_2026"} {
		if strings.Contains(content, secret) {
			t.Errorf("docker-compose.yml contains hardcoded secret default: %s", secret)
		}
	}
}

func TestDeployment_DockerfileStructure(t *testing.T) {
	apiDocker, err := os.ReadFile("../../deployments/docker/Dockerfile.api")
	if err != nil {
		t.Fatalf("failed to read Dockerfile.api: %v", err)
	}

	if !strings.Contains(string(apiDocker), "FROM golang:") || !strings.Contains(string(apiDocker), "ENTRYPOINT") {
		t.Errorf("Dockerfile.api does not appear to be a valid multi-stage Dockerfile")
	}
}

func TestDeployment_HealthProbeLifecycle(t *testing.T) {
	_ = os.Setenv("PG_SERVER_PORT", "8099")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load deployment config: %v", err)
	}

	h := health.NewHandler()

	// Live check
	liveReq := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	liveRec := httptest.NewRecorder()
	h.Live(liveRec, liveReq)

	if liveRec.Code != http.StatusOK {
		t.Errorf("expected 200 for live check, got %d", liveRec.Code)
	}

	// Ready check
	readyReq := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	readyRec := httptest.NewRecorder()
	h.Ready(readyRec, readyReq)

	if readyRec.Code != http.StatusOK {
		t.Errorf("expected 200 for ready check, got %d", readyRec.Code)
	}

	if cfg.Server.Port != 8099 {
		t.Errorf("expected configured port 8099, got %d", cfg.Server.Port)
	}
}
