package goobs

import (
	"context"
	"errors"
	"runtime"
	"testing"

	otellog "go.opentelemetry.io/otel/log"
	"go.uber.org/zap"
)

func TestLog_BuilderChain(t *testing.T) {
	b := Log()

	if got := b.FromContext(context.Background()); got != b {
		t.Error("FromContext should return same pointer")
	}
	if got := b.Debug(); got != b {
		t.Error("Debug should return same pointer")
	}
	if got := b.Info(); got != b {
		t.Error("Info should return same pointer")
	}
	if got := b.Warn(); got != b {
		t.Error("Warn should return same pointer")
	}
	if got := b.Error(); got != b {
		t.Error("Error should return same pointer")
	}
	if got := b.Msg("test"); got != b {
		t.Error("Msg should return same pointer")
	}
	if got := b.Field("key", "val"); got != b {
		t.Error("Field should return same pointer")
	}
	if got := b.Err(errors.New("test")); got != b {
		t.Error("Err should return same pointer")
	}
	if got := b.Fields(zap.String("k", "v")); got != b {
		t.Error("Fields should return same pointer")
	}
}

func TestLog_FromContext_NilSafe(t *testing.T) {
	b := Log()
	orig := b.ctx
	b.FromContext(nil)
	if b.ctx != orig {
		t.Error("FromContext(nil) should not overwrite ctx")
	}
}

func TestLog_FieldTypes(t *testing.T) {
	b := Log()
	b.Field("str", "hello").
		Field("int", 42).
		Field("int64", int64(100)).
		Field("float", 3.14).
		Field("bool", true).
		Field("other", struct{ X int }{X: 1})

	if len(b.fields) != 6 {
		t.Errorf("expected 6 fields, got %d", len(b.fields))
	}
}

func TestLog_Err_NilError(t *testing.T) {
	b := Log()
	b.Err(nil)
	if len(b.fields) != 0 {
		t.Error("Err(nil) should not add any field")
	}
}

func TestLog_Err_WithError(t *testing.T) {
	b := Log()
	b.Err(errors.New("something failed"))
	if len(b.fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(b.fields))
	}
	if b.fields[0].Key != "error" {
		t.Errorf("expected key 'error', got %q", b.fields[0].Key)
	}
}

func TestLog_Send_EmitsToOTel(t *testing.T) {
	env := setupTestEnv(t, Config{
		ServiceName: "test-svc",
		Environment: "test",
	})

	Log().Info().Msg("hello otel").Field("user", "alice").Send()

	records := env.LogExporter.Records()
	if len(records) == 0 {
		t.Fatal("expected at least 1 OTel log record")
	}

	rec := records[0]
	if rec.Body().AsString() != "hello otel" {
		t.Errorf("expected body 'hello otel', got %q", rec.Body().AsString())
	}
	if rec.Severity() != otellog.SeverityInfo {
		t.Errorf("expected severity INFO, got %v", rec.Severity())
	}
}

func TestLog_Send_EmptyMessage(t *testing.T) {
	env := setupTestEnv(t, Config{
		ServiceName: "test-svc",
		Environment: "test",
	})

	Log().Info().Send()

	records := env.LogExporter.Records()
	if len(records) == 0 {
		t.Fatal("expected at least 1 OTel log record")
	}
	if records[0].Body().AsString() != "no-message" {
		t.Errorf("expected body 'no-message', got %q", records[0].Body().AsString())
	}
}

func TestLog_Send_AllLevels(t *testing.T) {
	env := setupTestEnv(t, Config{
		ServiceName: "test-svc",
		Environment: "test",
	})

	tests := []struct {
		name     string
		setLevel func(b *LogObs) *LogObs
		expected otellog.Severity
	}{
		{"debug", (*LogObs).Debug, otellog.SeverityDebug},
		{"info", (*LogObs).Info, otellog.SeverityInfo},
		{"warn", (*LogObs).Warn, otellog.SeverityWarn},
		{"error", (*LogObs).Error, otellog.SeverityError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env.LogExporter.records = nil
			b := Log()
			tt.setLevel(b).Msg(tt.name).Send()

			records := env.LogExporter.Records()
			if len(records) == 0 {
				t.Fatal("expected log record")
			}
			if records[0].Severity() != tt.expected {
				t.Errorf("expected severity %v, got %v", tt.expected, records[0].Severity())
			}
		})
	}
}

func TestLog_Send_NoInit_NoPanic(t *testing.T) {
	// Ensure globals are nil
	initMu.Lock()
	globalOtelLogger = nil
	globalLogger = nil
	initialized = false
	initMu.Unlock()

	t.Cleanup(func() {
		initMu.Lock()
		initialized = false
		initMu.Unlock()
	})

	// Should not panic
	Log().Info().Msg("no init").Send()
}

func TestLog_SeverityText(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{levelDebug, "DEBUG"},
		{levelInfo, "INFO"},
		{levelWarn, "WARN"},
		{levelError, "ERROR"},
		{LogLevel(99), "INFO"},
	}

	for _, tt := range tests {
		b := &LogObs{level: tt.level}
		if got := b.severityText(); got != tt.expected {
			t.Errorf("level %d: expected %q, got %q", tt.level, tt.expected, got)
		}
	}
}

func TestLog_OtelSeverity(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected otellog.Severity
	}{
		{levelDebug, otellog.SeverityDebug},
		{levelInfo, otellog.SeverityInfo},
		{levelWarn, otellog.SeverityWarn},
		{levelError, otellog.SeverityError},
		{LogLevel(99), otellog.SeverityInfo},
	}

	for _, tt := range tests {
		b := &LogObs{level: tt.level}
		if got := b.otelSeverity(); got != tt.expected {
			t.Errorf("level %d: expected %v, got %v", tt.level, tt.expected, got)
		}
	}
}

func TestZapFieldsToOtelAttrs(t *testing.T) {
	fields := []zap.Field{
		zap.String("str", "hello"),
		zap.Bool("flag", true),
		zap.Int64("count", 42),
		zap.Error(errors.New("test error")),
	}

	attrs := zapFieldsToOtelAttrs(fields)

	if len(attrs) != 4 {
		t.Fatalf("expected 4 attrs, got %d", len(attrs))
	}

	if attrs[0].Value.AsString() != "hello" {
		t.Errorf("expected 'hello', got %q", attrs[0].Value.AsString())
	}
	if attrs[1].Value.AsBool() != true {
		t.Error("expected true for bool attr")
	}
	if attrs[2].Value.AsInt64() != 42 {
		t.Errorf("expected 42, got %d", attrs[2].Value.AsInt64())
	}
	// Error field should be converted to string
	if attrs[3].Value.AsString() == "" {
		t.Error("expected non-empty string for error attr")
	}
}

func TestShortFuncName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"main.Foo", "main.Foo"},
		{"github.com/user/pkg.Func", "pkg.Func"},
		{"github.com/user/pkg/sub.Func", "sub.Func"},
	}

	for _, tt := range tests {
		if got := shortFuncName(tt.input); got != tt.expected {
			t.Errorf("shortFuncName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestLogCaller_ReturnsNonEmpty(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test"})

	caller := logCaller()
	if caller == "" {
		t.Error("expected non-empty caller")
	}
}

func TestUseFrame_SkipCallerPkgs(t *testing.T) {
	_ = setupTestEnv(t, Config{
		ServiceName:    "test",
		SkipCallerPkgs: []string{"github.com/devloptech/goobs"},
	})

	frame := runtime.Frame{
		File:     "/Users/devdog/GolandProjects/goobs/test.go",
		Function: "github.com/devloptech/goobs.TestFunc",
	}

	if useFrame(frame) {
		t.Error("expected frame to be skipped by SkipCallerPkgs")
	}
}

func TestUseFrame_SkipCallerFiles(t *testing.T) {
	_ = setupTestEnv(t, Config{
		ServiceName:     "test",
		SkipCallerFiles: []string{"middleware.go"},
	})

	frame := runtime.Frame{
		File:     "/app/middleware.go",
		Function: "main.Handler",
	}

	if useFrame(frame) {
		t.Error("expected frame to be skipped by SkipCallerFiles")
	}
}

func TestUseFrame_EmptyFrame(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test"})

	if useFrame(runtime.Frame{}) {
		t.Error("expected empty frame to be skipped")
	}
}

func TestUseFrame_NormalFrame(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test"})

	frame := runtime.Frame{
		File:     "/app/handler.go",
		Function: "main.HandleRequest",
	}

	if !useFrame(frame) {
		t.Error("expected normal frame to be accepted")
	}
}
