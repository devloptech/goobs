package goobs

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func TestTrace_BuilderChain(t *testing.T) {
	b := Trace()

	if got := b.Name("test"); got != b {
		t.Error("Name should return same pointer")
	}
	if got := b.FromContext(context.Background()); got != b {
		t.Error("FromContext should return same pointer")
	}
	if got := b.Kind(trace.SpanKindServer); got != b {
		t.Error("Kind should return same pointer")
	}
	if got := b.TracerName("custom"); got != b {
		t.Error("TracerName should return same pointer")
	}
	if got := b.Attr("key", "val"); got != b {
		t.Error("Attr should return same pointer")
	}
	if got := b.Attrs(attribute.String("k", "v")); got != b {
		t.Error("Attrs should return same pointer")
	}
	if got := b.RecordError(false); got != b {
		t.Error("RecordError should return same pointer")
	}
	if got := b.SetStatusOnError(false); got != b {
		t.Error("SetStatusOnError should return same pointer")
	}
}

func TestTrace_Defaults(t *testing.T) {
	b := Trace()

	if b.kind != trace.SpanKindInternal {
		t.Errorf("expected default kind Internal, got %v", b.kind)
	}
	if b.recordErr != true {
		t.Error("expected recordErr to default true")
	}
	if b.setStatus != true {
		t.Error("expected setStatus to default true")
	}
	if b.tracerName != "goobs" {
		t.Errorf("expected tracerName 'goobs', got %q", b.tracerName)
	}
}

func TestTrace_FromContext_NilSafe(t *testing.T) {
	b := Trace()
	orig := b.ctx
	b.FromContext(nil)
	if b.ctx != orig {
		t.Error("FromContext(nil) should not overwrite ctx")
	}
}

func TestTrace_TracerName_EmptyIgnored(t *testing.T) {
	b := Trace()
	b.TracerName("")
	if b.tracerName != "goobs" {
		t.Error("empty TracerName should be ignored")
	}
}

func TestTrace_AttrTypes(t *testing.T) {
	b := Trace()
	b.Attr("str", "hello").
		Attr("int", 42).
		Attr("int64", int64(100)).
		Attr("float", 3.14).
		Attr("bool", true).
		Attr("other", struct{}{})

	if len(b.attrs) != 6 {
		t.Errorf("expected 6 attrs, got %d", len(b.attrs))
	}
}

func TestTrace_Start(t *testing.T) {
	env := setupTestEnv(t, Config{ServiceName: "test-svc"})
	_ = env

	ctx, span := Trace().Name("test-span").Start()
	defer span.End()

	if !span.SpanContext().IsValid() {
		t.Error("expected valid span context")
	}
	if ctx == nil {
		t.Error("expected non-nil context")
	}
}

func TestTrace_Start_UnnamedSpan(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	_, span := Trace().Start()
	span.End()

	// Should not panic, name defaults to "unnamed-span"
}

func TestTrace_StartScope(t *testing.T) {
	env := setupTestEnv(t, Config{ServiceName: "test-svc"})
	_ = env

	scope := Trace().Name("scope-span").StartScope()
	defer scope.Done()

	if scope.Ctx() == nil {
		t.Error("expected non-nil context from scope")
	}
	if scope.Span() == nil {
		t.Error("expected non-nil span from scope")
	}
	if !scope.Span().SpanContext().IsValid() {
		t.Error("expected valid span context from scope")
	}
}

func TestSpanScope_Nil_Safe(t *testing.T) {
	var scope *SpanScope

	// Should not panic
	scope.Done()

	if scope.Ctx() == nil {
		t.Error("Ctx() on nil scope should return background context")
	}
}

func TestTrace_Run_Success(t *testing.T) {
	env := setupTestEnv(t, Config{ServiceName: "test-svc"})

	called := false
	err := Trace().Name("run-ok").Run(func(ctx context.Context) error {
		called = true
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !called {
		t.Error("expected function to be called")
	}

	spans := env.SpanExporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least 1 span")
	}
	if spans[0].Name != "run-ok" {
		t.Errorf("expected span name 'run-ok', got %q", spans[0].Name)
	}
}

func TestTrace_Run_Error(t *testing.T) {
	env := setupTestEnv(t, Config{ServiceName: "test-svc"})

	testErr := errors.New("something failed")
	err := Trace().Name("run-err").Run(func(ctx context.Context) error {
		return testErr
	})

	if err != testErr {
		t.Errorf("expected testErr, got %v", err)
	}

	spans := env.SpanExporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least 1 span")
	}

	span := spans[0]
	if span.Status.Code != codes.Error {
		t.Errorf("expected error status, got %v", span.Status.Code)
	}
	if len(span.Events) == 0 {
		t.Error("expected error event recorded on span")
	}
}

func TestTrace_Run_NilFn(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	err := Trace().Name("nil-fn").Run(nil)
	if err == nil {
		t.Error("expected error for nil fn")
	}
	if err.Error() != "goobs.Trace().Run: fn is nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestTrace_Run_NoRecordError(t *testing.T) {
	env := setupTestEnv(t, Config{ServiceName: "test-svc"})

	_ = Trace().Name("no-record").RecordError(false).SetStatusOnError(false).Run(func(ctx context.Context) error {
		return errors.New("ignored error")
	})

	spans := env.SpanExporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected span")
	}

	span := spans[0]
	if span.Status.Code == codes.Error {
		t.Error("expected no error status when SetStatusOnError is false")
	}
	if len(span.Events) != 0 {
		t.Error("expected no events when RecordError is false")
	}
}

func TestTrace_Start_WithAttrs(t *testing.T) {
	env := setupTestEnv(t, Config{ServiceName: "test-svc"})

	_, span := Trace().
		Name("attrs-span").
		Attr("key1", "val1").
		Attr("key2", 42).
		Start()
	span.End()

	spans := env.SpanExporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected span")
	}

	attrs := spans[0].Attributes
	if len(attrs) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(attrs))
	}
}

func TestTrace_Start_WithKind(t *testing.T) {
	env := setupTestEnv(t, Config{ServiceName: "test-svc"})

	_, span := Trace().
		Name("server-span").
		Kind(trace.SpanKindServer).
		Start()
	span.End()

	spans := env.SpanExporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected span")
	}
	if spans[0].SpanKind != trace.SpanKindServer {
		t.Errorf("expected SpanKindServer, got %v", spans[0].SpanKind)
	}
}
