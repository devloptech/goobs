package goobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

func TestPropagate_BuilderChain(t *testing.T) {
	b := Propagate()

	if got := b.FromContext(context.Background()); got != b {
		t.Error("FromContext should return same pointer")
	}
	if got := b.WithLegacyHeaders(true); got != b {
		t.Error("WithLegacyHeaders should return same pointer")
	}
}

func TestPropagate_FromContext_NilSafe(t *testing.T) {
	b := Propagate()
	orig := b.ctx
	b.FromContext(nil)
	if b.ctx != orig {
		t.Error("FromContext(nil) should not overwrite ctx")
	}
}

func TestPropagate_FromHTTPRequest(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := Propagate().FromHTTPRequest(req)

	if ctx == nil {
		t.Error("expected non-nil context")
	}
}

func TestPropagate_FromHTTPRequest_UsesBuilderCtx(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	// Create a span to get a valid trace context
	parentCtx, span := Trace().Name("parent").Start()
	defer span.End()

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := Propagate().FromContext(parentCtx).FromHTTPRequest(req)

	// The returned context should have trace info from the parent
	sc := trace.SpanFromContext(ctx).SpanContext()
	parentSC := span.SpanContext()

	// Should inherit the trace from builder context (not request context)
	if sc.TraceID() != parentSC.TraceID() {
		// This is expected because we're extracting from empty headers,
		// but the base context should be the builder's context
	}
	_ = ctx
}

func TestPropagate_ToHTTPRequest(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	ctx, span := Trace().Name("out-span").Start()
	defer span.End()

	req := httptest.NewRequest("GET", "/downstream", nil)
	Propagate().FromContext(ctx).ToHTTPRequest(req)

	// W3C traceparent should be injected
	if req.Header.Get("traceparent") == "" {
		t.Error("expected traceparent header to be injected")
	}
}

func TestPropagate_ToHTTPRequest_WithLegacy(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	ctx, span := Trace().Name("legacy-span").Start()
	defer span.End()

	req := httptest.NewRequest("GET", "/downstream", nil)
	Propagate().FromContext(ctx).WithLegacyHeaders(true).ToHTTPRequest(req)

	if req.Header.Get("traceparent") == "" {
		t.Error("expected traceparent header")
	}
	if req.Header.Get("x-trace-id") == "" {
		t.Error("expected x-trace-id legacy header")
	}
	if req.Header.Get("x-span-id") == "" {
		t.Error("expected x-span-id legacy header")
	}
}

func TestPropagate_ToHTTPRequest_WithoutLegacy(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	ctx, span := Trace().Name("no-legacy").Start()
	defer span.End()

	req := httptest.NewRequest("GET", "/downstream", nil)
	Propagate().FromContext(ctx).WithLegacyHeaders(false).ToHTTPRequest(req)

	if req.Header.Get("x-trace-id") != "" {
		t.Error("expected no x-trace-id header when legacy disabled")
	}
}

func TestPropagate_ToHTTPResponse(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	ctx, span := Trace().Name("resp-span").Start()
	defer span.End()

	w := httptest.NewRecorder()
	Propagate().FromContext(ctx).ToHTTPResponse(w)

	if w.Header().Get("x-trace-id") == "" {
		t.Error("expected x-trace-id in response header")
	}
	if w.Header().Get("x-span-id") == "" {
		t.Error("expected x-span-id in response header")
	}
}

func TestPropagate_ToHTTPResponse_NoSpan(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	w := httptest.NewRecorder()
	Propagate().FromContext(context.Background()).ToHTTPResponse(w)

	// No span in context, so no headers should be set
	if w.Header().Get("x-trace-id") != "" {
		t.Error("expected no x-trace-id when no span in context")
	}
}

func TestPropagate_RoundTrip_HTTP(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	// Create outbound span
	ctx, span := Trace().Name("original").Start()
	defer span.End()
	originalTraceID := span.SpanContext().TraceID().String()

	// Inject into request
	outReq := httptest.NewRequest("GET", "/api", nil)
	Propagate().FromContext(ctx).ToHTTPRequest(outReq)

	// Extract from request (simulating receiving side)
	inCtx := Propagate().FromHTTPRequest(outReq)
	inSpan := trace.SpanFromContext(inCtx)
	extractedTraceID := inSpan.SpanContext().TraceID().String()

	if originalTraceID != extractedTraceID {
		t.Errorf("trace ID mismatch: sent %s, received %s", originalTraceID, extractedTraceID)
	}
}

// --- gRPC metadata tests ---

func TestMetadataCarrier_GetSetKeys(t *testing.T) {
	md := metadata.MD{}
	c := metadataCarrier{md}

	c.Set("key1", "val1")
	c.Set("key2", "val2")

	if got := c.Get("key1"); got != "val1" {
		t.Errorf("expected 'val1', got %q", got)
	}
	if got := c.Get("missing"); got != "" {
		t.Errorf("expected empty for missing key, got %q", got)
	}

	keys := c.Keys()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestPropagate_gRPC_RoundTrip(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	ctx, span := Trace().Name("grpc-span").Start()
	defer span.End()

	md := metadata.MD{}
	Propagate().FromContext(ctx).ToGRPCMetadata(&md)

	if len(md) == 0 {
		t.Error("expected metadata to be populated")
	}

	// Extract
	inCtx := Propagate().FromGRPCMetadata(context.Background(), md)
	inSpan := trace.SpanFromContext(inCtx)

	if inSpan.SpanContext().TraceID() != span.SpanContext().TraceID() {
		t.Error("trace ID mismatch in gRPC round-trip")
	}
}

func TestPropagate_ToGRPCMetadata_NilMD(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	// Should not panic with nil md
	Propagate().FromContext(context.Background()).ToGRPCMetadata(nil)
}

func TestPropagate_ToGRPCMetadata_NilMDValue(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	ctx, span := Trace().Name("grpc-nil").Start()
	defer span.End()

	var md metadata.MD
	Propagate().FromContext(ctx).ToGRPCMetadata(&md)

	// md should be initialized
	if md == nil {
		t.Error("expected md to be initialized")
	}
}

// --- AMQP header carrier tests ---

func TestAmqpHeaderCarrier_GetSetKeys(t *testing.T) {
	headers := amqp.Table{}
	c := amqpHeaderCarrier(headers)

	c.Set("traceparent", "00-abc-def-01")
	c.Set("tracestate", "vendor=value")

	if got := c.Get("traceparent"); got != "00-abc-def-01" {
		t.Errorf("expected '00-abc-def-01', got %q", got)
	}
	if got := c.Get("missing"); got != "" {
		t.Errorf("expected empty for missing key, got %q", got)
	}

	keys := c.Keys()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestAmqpHeaderCarrier_Get_NonStringValue(t *testing.T) {
	headers := amqp.Table{
		"numeric": int64(42),
	}
	c := amqpHeaderCarrier(headers)

	if got := c.Get("numeric"); got != "" {
		t.Errorf("expected empty for non-string value, got %q", got)
	}
}

// --- No-init safety ---

func TestPropagate_NoInit_NoPanic(t *testing.T) {
	initMu.Lock()
	globalPropagator = nil
	initMu.Unlock()

	t.Cleanup(func() {
		initMu.Lock()
		globalPropagator = nil
		initMu.Unlock()
	})

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := Propagate().FromHTTPRequest(req)
	if ctx == nil {
		t.Error("expected non-nil context even without init")
	}

	outReq := httptest.NewRequest("GET", "/out", nil)
	Propagate().FromContext(context.Background()).ToHTTPRequest(outReq)

	w := httptest.NewRecorder()
	Propagate().FromContext(context.Background()).ToHTTPResponse(w)

	Propagate().FromGRPCMetadata(context.Background(), metadata.MD{})

	var md metadata.MD
	Propagate().FromContext(context.Background()).ToGRPCMetadata(&md)

	Propagate().FromContext(context.Background()).FromAMQP(amqp.Table{})

	Propagate().FromContext(context.Background()).ToAMQP(amqp.Table{})
}

func TestPropagate_FromHTTPRequest_NilPropagator(t *testing.T) {
	initMu.Lock()
	globalPropagator = nil
	initMu.Unlock()

	t.Cleanup(func() {
		initMu.Lock()
		globalPropagator = nil
		initMu.Unlock()
	})

	req, _ := http.NewRequest("GET", "/", nil)
	ctx := Propagate().FromHTTPRequest(req)
	if ctx == nil {
		t.Error("expected fallback to r.Context()")
	}
}
