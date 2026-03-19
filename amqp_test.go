package goobs

import (
	"context"
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/trace"
)

func TestPropagate_AMQP_RoundTrip(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	ctx, span := Trace().Name("amqp-span").Start()
	defer span.End()

	headers := amqp.Table{}
	Propagate().FromContext(ctx).ToAMQP(headers)

	if len(headers) == 0 {
		t.Error("expected headers to be populated")
	}

	// Extract
	inCtx := Propagate().FromContext(context.Background()).FromAMQP(headers)
	inSpan := trace.SpanFromContext(inCtx)

	if inSpan.SpanContext().TraceID() != span.SpanContext().TraceID() {
		t.Error("trace ID mismatch in AMQP round-trip")
	}
}

func TestPropagate_ToAMQP_WithLegacy(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	ctx, span := Trace().Name("amqp-legacy").Start()
	defer span.End()

	headers := amqp.Table{}
	Propagate().FromContext(ctx).WithLegacyHeaders(true).ToAMQP(headers)

	if _, ok := headers["x-trace-id"]; !ok {
		t.Error("expected x-trace-id in AMQP headers with legacy enabled")
	}
	if _, ok := headers["x-span-id"]; !ok {
		t.Error("expected x-span-id in AMQP headers with legacy enabled")
	}
}

func TestPropagate_ToAMQP_WithoutLegacy(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	ctx, span := Trace().Name("amqp-no-legacy").Start()
	defer span.End()

	headers := amqp.Table{}
	Propagate().FromContext(ctx).WithLegacyHeaders(false).ToAMQP(headers)

	if _, ok := headers["x-trace-id"]; ok {
		t.Error("expected no x-trace-id when legacy disabled")
	}
}

func TestAMQPConsumerInterceptor_Success(t *testing.T) {
	env := setupTestEnv(t, Config{
		ServiceName:   "test-svc",
		EnableMetrics: true,
	})

	called := false
	handler := AMQPConsumerInterceptor("test-svc", func(ctx context.Context, msg amqp.Delivery) error {
		called = true

		// Verify span is active in context
		span := trace.SpanFromContext(ctx)
		if !span.SpanContext().IsValid() {
			t.Error("expected valid span in handler context")
		}
		return nil
	})

	msg := amqp.Delivery{
		RoutingKey: "test.queue",
		Exchange:   "test-exchange",
		Headers:    amqp.Table{},
	}

	handler(msg)

	if !called {
		t.Error("expected handler to be called")
	}

	// Verify span was created
	spans := env.SpanExporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least 1 span from interceptor")
	}

	span := spans[0]
	if span.Name != "amqp.consume" {
		t.Errorf("expected span name 'amqp.consume', got %q", span.Name)
	}

	// Check attributes
	foundQueue := false
	foundExchange := false
	for _, attr := range span.Attributes {
		if string(attr.Key) == "amqp.queue" && attr.Value.AsString() == "test.queue" {
			foundQueue = true
		}
		if string(attr.Key) == "amqp.exchange" && attr.Value.AsString() == "test-exchange" {
			foundExchange = true
		}
	}
	if !foundQueue {
		t.Error("expected amqp.queue attribute on span")
	}
	if !foundExchange {
		t.Error("expected amqp.exchange attribute on span")
	}

	// Verify metrics
	rm := env.CollectMetrics(t)
	foundCounter := false
	foundHistogram := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "amqp_consume_total" {
				foundCounter = true
			}
			if m.Name == "amqp_consume_duration_ms" {
				foundHistogram = true
			}
		}
	}
	if !foundCounter {
		t.Error("expected amqp_consume_total metric")
	}
	if !foundHistogram {
		t.Error("expected amqp_consume_duration_ms metric")
	}
}

func TestAMQPConsumerInterceptor_Error(t *testing.T) {
	env := setupTestEnv(t, Config{
		ServiceName:   "test-svc",
		EnableMetrics: true,
	})

	handler := AMQPConsumerInterceptor("test-svc", func(ctx context.Context, msg amqp.Delivery) error {
		return errors.New("processing failed")
	})

	msg := amqp.Delivery{
		RoutingKey: "error.queue",
		Exchange:   "",
		Headers:    amqp.Table{},
	}

	handler(msg)

	// Verify span recorded error
	spans := env.SpanExporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected span")
	}

	// The Run() method should have recorded the error
	if len(spans[0].Events) == 0 {
		t.Error("expected error event on span")
	}
}

func TestAMQPConsumerInterceptor_PropagatesContext(t *testing.T) {
	_ = setupTestEnv(t, Config{ServiceName: "test-svc"})

	// Create a parent span and inject into AMQP headers
	parentCtx, parentSpan := Trace().Name("parent-publish").Start()
	defer parentSpan.End()

	headers := amqp.Table{}
	Propagate().FromContext(parentCtx).ToAMQP(headers)

	var receivedTraceID string
	handler := AMQPConsumerInterceptor("test-svc", func(ctx context.Context, msg amqp.Delivery) error {
		span := trace.SpanFromContext(ctx)
		receivedTraceID = span.SpanContext().TraceID().String()
		return nil
	})

	msg := amqp.Delivery{
		RoutingKey: "test.queue",
		Exchange:   "test-exchange",
		Headers:    headers,
	}

	handler(msg)

	expectedTraceID := parentSpan.SpanContext().TraceID().String()
	if receivedTraceID != expectedTraceID {
		t.Errorf("expected trace ID %s to propagate, got %s", expectedTraceID, receivedTraceID)
	}
}
