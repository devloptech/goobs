package goobs

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type testEnv struct {
	SpanExporter  *tracetest.InMemoryExporter
	MetricReader  *sdkmetric.ManualReader
	LogExporter   *inMemoryLogExporter
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	LogProvider    *sdklog.LoggerProvider
	Logger         *zap.Logger
}

func setupTestEnv(t *testing.T, cfg Config) *testEnv {
	t.Helper()

	spanExp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spanExp))

	metricReader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricReader))

	logExp := newInMemoryLogExporter()
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(logExp)))

	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)

	zapLogger := zaptest.NewLogger(t)

	initMu.Lock()
	globalCfg = cfg
	globalTP = tp
	globalLogProvider = lp
	globalOtelLogger = lp.Logger(cfg.ServiceName)
	globalLogger = zapLogger
	globalPropagator = propagator
	if cfg.EnableMetrics {
		globalMP = mp
		globalMeter = mp.Meter(cfg.ServiceName)
	} else {
		globalMP = nil
		globalMeter = nil
	}
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagator)
	initialized = true
	initMu.Unlock()

	t.Cleanup(func() {
		initMu.Lock()
		defer initMu.Unlock()
		tp.Shutdown(context.Background())
		mp.Shutdown(context.Background())
		lp.Shutdown(context.Background())
		globalTP = nil
		globalMP = nil
		globalLogProvider = nil
		globalOtelLogger = nil
		globalLogger = nil
		globalPropagator = nil
		globalMeter = nil
		globalCfg = Config{}
		initialized = false

		counterMu.Lock()
		counterCache = map[string]metric.Int64Counter{}
		counterMu.Unlock()
		histogramMu.Lock()
		histogramCache = map[string]metric.Float64Histogram{}
		histogramMu.Unlock()
		gaugeMu.Lock()
		gaugeCache = map[string]metric.Float64Gauge{}
		gaugeMu.Unlock()
	})

	return &testEnv{
		SpanExporter:   spanExp,
		MetricReader:   metricReader,
		LogExporter:    logExp,
		TracerProvider: tp,
		MeterProvider:  mp,
		LogProvider:    lp,
		Logger:         zapLogger,
	}
}

func (e *testEnv) CollectMetrics(t *testing.T) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := e.MetricReader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}
	return rm
}

// inMemoryLogExporter collects OTel log records for testing.
type inMemoryLogExporter struct {
	records []sdklog.Record
}

func newInMemoryLogExporter() *inMemoryLogExporter {
	return &inMemoryLogExporter{}
}

func (e *inMemoryLogExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.records = append(e.records, records...)
	return nil
}

func (e *inMemoryLogExporter) Shutdown(_ context.Context) error { return nil }

func (e *inMemoryLogExporter) ForceFlush(_ context.Context) error { return nil }

func (e *inMemoryLogExporter) Records() []sdklog.Record {
	return e.records
}
