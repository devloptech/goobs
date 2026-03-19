package goobs

import (
	"context"
	"testing"
)

func TestInit_SetsGlobals(t *testing.T) {
	env := setupTestEnv(t, Config{
		ServiceName:   "test-svc",
		Environment:   "test",
		OtelEndpoint:  "localhost:4317",
		EnableMetrics: true,
	})

	cfg, otelLogger, zapLogger, prop, meter := getGlobals()

	if cfg.ServiceName != "test-svc" {
		t.Errorf("expected ServiceName 'test-svc', got %q", cfg.ServiceName)
	}
	if otelLogger == nil {
		t.Error("expected otelLogger to be set")
	}
	if zapLogger == nil {
		t.Error("expected zapLogger to be set")
	}
	if prop == nil {
		t.Error("expected propagator to be set")
	}
	if meter == nil {
		t.Error("expected meter to be set")
	}
	_ = env
}

func TestInit_MetricsDisabled(t *testing.T) {
	_ = setupTestEnv(t, Config{
		ServiceName:   "test-svc",
		Environment:   "test",
		OtelEndpoint:  "localhost:4317",
		EnableMetrics: false,
	})

	_, _, _, _, meter := getGlobals()
	if meter != nil {
		t.Error("expected meter to be nil when metrics disabled")
	}
}

func TestInit_Idempotent(t *testing.T) {
	_ = setupTestEnv(t, Config{
		ServiceName:   "first",
		Environment:   "test",
		OtelEndpoint:  "localhost:4317",
		EnableMetrics: true,
	})

	// Re-setup with different config should succeed
	_ = setupTestEnv(t, Config{
		ServiceName:   "second",
		Environment:   "test",
		OtelEndpoint:  "localhost:4317",
		EnableMetrics: true,
	})

	cfg, _, _, _, _ := getGlobals()
	if cfg.ServiceName != "second" {
		t.Errorf("expected ServiceName 'second' after re-init, got %q", cfg.ServiceName)
	}
}

func TestShutdown_CleansGlobals(t *testing.T) {
	_ = setupTestEnv(t, Config{
		ServiceName:   "test-svc",
		Environment:   "test",
		OtelEndpoint:  "localhost:4317",
		EnableMetrics: true,
	})

	// Manually trigger cleanup (simulates what t.Cleanup does)
	initMu.Lock()
	err := shutdownProviders(context.Background())
	initialized = false
	initMu.Unlock()

	if err != nil {
		t.Errorf("shutdown returned error: %v", err)
	}

	_, otelLogger, zapLogger, prop, meter := getGlobals()
	if otelLogger != nil {
		t.Error("expected otelLogger to be nil after shutdown")
	}
	if zapLogger != nil {
		t.Error("expected zapLogger to be nil after shutdown")
	}
	if prop != nil {
		t.Error("expected propagator to be nil after shutdown")
	}
	if meter != nil {
		t.Error("expected meter to be nil after shutdown")
	}
}
