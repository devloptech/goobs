package goobs

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/trace"
)

type AMQPConsumeHandler func(ctx context.Context, msg amqp.Delivery) error

func AMQPConsumerInterceptor(serviceName string, handler AMQPConsumeHandler) func(msg amqp.Delivery) {
	return func(msg amqp.Delivery) {
		baseCtx := context.Background()

		ctx := Propagate().
			FromContext(baseCtx).
			FromAMQP(msg.Headers)

		_ = Trace().
			Name("amqp.consume").
			FromContext(ctx).
			Kind(trace.SpanKindConsumer).
			Attr("amqp.queue", msg.RoutingKey).
			Attr("amqp.exchange", msg.Exchange).
			Run(func(ctx context.Context) error {
				start := time.Now()

				err := handler(ctx, msg)

				status := "success"
				if err != nil {
					status = "error"
				}

				MetricCounter("amqp_consume_total").
					Attr("service", serviceName).
					Attr("queue", msg.RoutingKey).
					Attr("status", status).
					Add(ctx, 1)

				latencyMs := float64(time.Since(start).Milliseconds())
				MetricHistogram("amqp_consume_duration_ms").
					Attr("service", serviceName).
					Attr("queue", msg.RoutingKey).
					Attr("status", status).
					Record(ctx, latencyMs)

				return err
			})
	}
}

func (p *PropagationObs) FromAMQP(headers amqp.Table) context.Context {
	_, _, _, prop, _ := getGlobals()
	if prop == nil {
		return p.ctx
	}
	carrier := amqpHeaderCarrier(headers)
	return prop.Extract(p.ctx, carrier)
}

func (p *PropagationObs) ToAMQP(headers amqp.Table) {
	_, _, _, prop, _ := getGlobals()
	if prop == nil {
		return
	}
	carrier := amqpHeaderCarrier(headers)
	prop.Inject(p.ctx, carrier)

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

	headers["x-trace-id"] = sc.TraceID().String()
	headers["x-span-id"] = sc.SpanID().String()
}
