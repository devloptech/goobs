package goobs

import (
	"context"
	"net/http"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

type PropagationObs struct {
	ctx       context.Context
	useLegacy bool
	err       interface{}
}

func Propagate() *PropagationObs {
	return &PropagationObs{
		ctx: context.Background(),
	}
}

func (p *PropagationObs) FromContext(ctx context.Context) *PropagationObs {
	if ctx != nil {
		p.ctx = ctx
	}
	return p
}

func (p *PropagationObs) WithLegacyHeaders(enable bool) *PropagationObs {
	p.useLegacy = enable
	return p
}

// ---------- HTTP Inbound ----------
func (p *PropagationObs) FromHTTPRequest(r *http.Request) context.Context {
	if globalPropagator == nil {
		return r.Context()
	}
	return globalPropagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
}

// ---------- HTTP Outbound ----------
func (p *PropagationObs) ToHTTPRequest(r *http.Request) {
	if globalPropagator == nil {
		return
	}
	globalPropagator.Inject(p.ctx, propagation.HeaderCarrier(r.Header))

	if !p.useLegacy {
		return
	}

	span := trace.SpanFromContext(p.ctx)
	if span == nil {
		return
	}
	sc := span.SpanContext()
	if !sc.IsValid() {
		return
	}

	r.Header.Set("x-trace-id", sc.TraceID().String())
	r.Header.Set("x-span-id", sc.SpanID().String())
}

// ---------- HTTP Response ----------
func (p *PropagationObs) ToHTTPResponse(w http.ResponseWriter) {
	span := trace.SpanFromContext(p.ctx)
	if span == nil {
		return
	}
	sc := span.SpanContext()
	if !sc.IsValid() {
		return
	}
	w.Header().Set("x-trace-id", sc.TraceID().String())
	w.Header().Set("x-span-id", sc.SpanID().String())
}

// ---------- gRPC (optional) ----------
type metadataCarrier struct {
	metadata.MD
}

func (c metadataCarrier) Get(key string) string {
	vals := c.MD.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (c metadataCarrier) Set(key, val string) {
	c.MD.Set(key, val)
}

func (c metadataCarrier) Keys() []string {
	out := make([]string, 0, len(c.MD))
	for k := range c.MD {
		out = append(out, k)
	}
	return out
}

func (p *PropagationObs) FromGRPCMetadata(ctx context.Context, md metadata.MD) context.Context {
	if globalPropagator == nil {
		return ctx
	}
	carrier := metadataCarrier{md}
	return globalPropagator.Extract(ctx, carrier)
}

func (p *PropagationObs) ToGRPCMetadata(md *metadata.MD) {
	if globalPropagator == nil || md == nil {
		return
	}
	if *md == nil {
		*md = metadata.MD{}
	}
	carrier := metadataCarrier{*md}
	globalPropagator.Inject(p.ctx, carrier)
}

// ---------- AMQP (RabbitMQ) ----------
type amqpHeaderCarrier amqp.Table

func (c amqpHeaderCarrier) Get(key string) string {
	if v, ok := c[key]; ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}

func (c amqpHeaderCarrier) Set(key, val string) {
	c[key] = val
}

func (c amqpHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}
