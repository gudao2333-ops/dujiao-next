package router

import (
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/provider"
)

func TestAdminSubsiteContractRoutesRegistered(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Mode = "test"
	cfg.CORS.AllowedOrigins = []string{"*"}
	cfg.CORS.AllowedMethods = config.DefaultCORSAllowedMethods()
	cfg.CORS.AllowedHeaders = config.DefaultCORSAllowedHeaders()
	cfg.Security.LoginRateLimit.WindowSeconds = 60
	cfg.Security.LoginRateLimit.MaxAttempts = 10
	cfg.Security.LoginRateLimit.BlockSeconds = 60

	r := SetupRouter(cfg, &provider.Container{})
	routes := map[string]bool{}
	for _, rt := range r.Routes() {
		routes[rt.Method+" "+rt.Path] = true
	}

	expected := []string{
		"GET /api/v1/admin/subsites/settings",
		"PUT /api/v1/admin/subsites/settings",
		"GET /api/v1/admin/subsites/suffixes",
		"POST /api/v1/admin/subsites/suffixes",
		"PUT /api/v1/admin/subsites/suffixes/:id",
		"DELETE /api/v1/admin/subsites/suffixes/:id",
		"GET /api/v1/admin/subsites",
		"PUT /api/v1/admin/subsites/:id/status",
		"GET /api/v1/admin/subsites/withdraws",
		"POST /api/v1/admin/subsites/withdraws/:id/reject",
		"POST /api/v1/admin/subsites/withdraws/:id/pay",
		"GET /api/v1/admin/settings/site-open",
		"GET /api/v1/admin/sites/withdraws",
	}
	for _, key := range expected {
		if !routes[key] {
			t.Fatalf("missing route: %s", key)
		}
	}
}
