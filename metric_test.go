package goobs

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestMetricCounter_BuilderChain(t *testing.T) {
	b := MetricCounter("test")

	if got := b.Attr("k", "v"); got != b {
		t.Error("Attr should return same pointer")
	}
	if got := b.Attrs(attribute.String("k", "v")); got != b {
		t.Error("Attrs should return same pointer")
	}
	if got := b.Unit("req"); got != b {
		t.Error("Unit should return same pointer")
	}
	if got := b.Description("desc"); got != b {
		t.Error("Description should return same pointer")
	}
}

func TestMetricHistogram_BuilderChain(t *testing.T) {
	b := MetricHistogram("test")

	if got := b.Attr("k", "v"); got != b {
		t.Error("Attr should return same pointer")
	}
	if got := b.Attrs(attribute.String("k", "v")); got != b {
		t.Error("Attrs should return same pointer")
	}
	if got := b.Unit("s"); got != b {
		t.Error("Unit should return same pointer")
	}
	if got := b.Description("desc"); got != b {
		t.Error("Description should return same pointer")
	}
}

func TestMetricGauge_BuilderChain(t *testing.T) {
	b := MetricGauge("test")

	if got := b.Attr("k", "v"); got != b {
		t.Error("Attr should return same pointer")
	}
	if got := b.Attrs(attribute.String("k", "v")); got != b {
		t.Error("Attrs should return same pointer")
	}
	if got := b.Unit("conn"); got != b {
		t.Error("Unit should return same pointer")
	}
	if got := b.Description("desc"); got != b {
		t.Error("Description should return same pointer")
	}
}

func TestMetricCounter_Defaults(t *testing.T) {
	b := MetricCounter("requests_total")
	if b.name != "requests_total" {
		t.Errorf("expected name 'requests_total', got %q", b.name)
	}
	if b.unit != "1" {
		t.Errorf("expected default unit '1', got %q", b.unit)
	}
}

func TestMetricHistogram_Defaults(t *testing.T) {
	b := MetricHistogram("duration")
	if b.name != "duration" {
		t.Errorf("expected name 'duration', got %q", b.name)
	}
	if b.unit != "ms" {
		t.Errorf("expected default unit 'ms', got %q", b.unit)
	}
}

func TestMetricGauge_Defaults(t *testing.T) {
	b := MetricGauge("connections")
	if b.name != "connections" {
		t.Errorf("expected name 'connections', got %q", b.name)
	}
	if b.unit != "1" {
		t.Errorf("expected default unit '1', got %q", b.unit)
	}
}

func TestMetricCounter_Unit_EmptyIgnored(t *testing.T) {
	b := MetricCounter("test")
	b.Unit("")
	if b.unit != "1" {
		t.Error("empty unit should be ignored")
	}
}

func TestMetricHistogram_Unit_EmptyIgnored(t *testing.T) {
	b := MetricHistogram("test")
	b.Unit("")
	if b.unit != "ms" {
		t.Error("empty unit should be ignored")
	}
}

func TestMetricGauge_Unit_EmptyIgnored(t *testing.T) {
	b := MetricGauge("test")
	b.Unit("")
	if b.unit != "1" {
		t.Error("empty unit should be ignored")
	}
}

func TestMetricCounter_NoOp_WhenDisabled(t *testing.T) {
	_ = setupTestEnv(t, Config{
		ServiceName:   "test-svc",
		EnableMetrics: false,
	})

	// Should not panic
	MetricCounter("test_counter").
		Attr("method", "GET").
		Add(context.Background(), 1)
}

func TestMetricHistogram_NoOp_WhenDisabled(t *testing.T) {
	_ = setupTestEnv(t, Config{
		ServiceName:   "test-svc",
		EnableMetrics: false,
	})

	// Should not panic
	MetricHistogram("test_hist").
		Attr("path", "/api").
		Record(context.Background(), 42.5)
}

func TestMetricGauge_NoOp_WhenDisabled(t *testing.T) {
	_ = setupTestEnv(t, Config{
		ServiceName:   "test-svc",
		EnableMetrics: false,
	})

	// Should not panic
	MetricGauge("test_gauge").
		Attr("type", "conn").
		Record(context.Background(), 10)
}

func TestMetricCounter_Add_WithMetrics(t *testing.T) {
	env := setupTestEnv(t, Config{
		ServiceName:   "test-svc",
		EnableMetrics: true,
	})

	MetricCounter("http_requests").
		Attr("method", "POST").
		Unit("req").
		Description("HTTP request count").
		Add(context.Background(), 5)

	rm := env.CollectMetrics(t)
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "http_requests" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected to find 'http_requests' metric")
	}
}

func TestMetricHistogram_Record_WithMetrics(t *testing.T) {
	env := setupTestEnv(t, Config{
		ServiceName:   "test-svc",
		EnableMetrics: true,
	})

	MetricHistogram("request_duration").
		Attr("path", "/api").
		Record(context.Background(), 150.5)

	rm := env.CollectMetrics(t)
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "request_duration" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected to find 'request_duration' metric")
	}
}

func TestMetricGauge_Record_WithMetrics(t *testing.T) {
	env := setupTestEnv(t, Config{
		ServiceName:   "test-svc",
		EnableMetrics: true,
	})

	MetricGauge("active_conns").
		Attr("service", "api").
		Record(context.Background(), 42)

	rm := env.CollectMetrics(t)
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "active_conns" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected to find 'active_conns' metric")
	}
}

func TestMetric_CacheByNameUnitDesc(t *testing.T) {
	_ = setupTestEnv(t, Config{
		ServiceName:   "test-svc",
		EnableMetrics: true,
	})

	// Same name, different unit should create different cache entries
	MetricCounter("ops").Unit("req").Add(context.Background(), 1)
	MetricCounter("ops").Unit("bytes").Add(context.Background(), 1)

	counterMu.Lock()
	count := len(counterCache)
	counterMu.Unlock()

	if count < 2 {
		t.Errorf("expected at least 2 cached counters (different units), got %d", count)
	}
}

func TestAnyToAttr(t *testing.T) {
	tests := []struct {
		name string
		val  any
	}{
		{"string", "hello"},
		{"int", 42},
		{"int64", int64(100)},
		{"float64", 3.14},
		{"bool", true},
		{"other", struct{ X int }{X: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := anyToAttr("key", tt.val)
			if string(attr.Key) != "key" {
				t.Errorf("expected key 'key', got %q", attr.Key)
			}
		})
	}
}

func TestMetricCounter_NoInit_NoPanic(t *testing.T) {
	initMu.Lock()
	globalMeter = nil
	globalCfg = Config{EnableMetrics: true}
	initMu.Unlock()

	t.Cleanup(func() {
		initMu.Lock()
		globalCfg = Config{}
		initMu.Unlock()
	})

	// Should not panic even with EnableMetrics but nil meter
	MetricCounter("test").Add(context.Background(), 1)
}
