package goobs

import (
	"context"
	"errors"
	"sync"

	"go.opentelemetry.io/otel"
	otlploggrpc "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	otlpmetricgrpc "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otlpgrpc "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log"
	logglobal "go.opentelemetry.io/otel/log/global"

	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"

	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

var (
	initMu            sync.RWMutex
	initialized       bool
	globalCfg         Config
	globalTP          *sdktrace.TracerProvider
	globalMP          *sdkmetric.MeterProvider
	globalLogProvider *sdklog.LoggerProvider
	globalOtelLogger  otellog.Logger
	globalLogger      *zap.Logger
	globalPropagator  propagation.TextMapPropagator
	globalMeter       metric.Meter
)

func getGlobals() (Config, otellog.Logger, *zap.Logger, propagation.TextMapPropagator, metric.Meter) {
	initMu.RLock()
	defer initMu.RUnlock()
	return globalCfg, globalOtelLogger, globalLogger, globalPropagator, globalMeter
}

func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	initMu.Lock()
	defer initMu.Unlock()

	if initialized {
		if err := shutdownProviders(ctx); err != nil {
			return nil, err
		}
	}

	globalCfg = cfg

	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	traceExp, err := otlpgrpc.New(
		ctx,
		otlpgrpc.WithEndpoint(cfg.OtelEndpoint),
		otlpgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	globalTP = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(globalTP)

	if cfg.EnableMetrics {
		metricExp, err := otlpmetricgrpc.New(
			ctx,
			otlpmetricgrpc.WithEndpoint(cfg.OtelEndpoint),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}

		reader := sdkmetric.NewPeriodicReader(metricExp)
		globalMP = sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(reader),
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(globalMP)
		globalMeter = globalMP.Meter(cfg.ServiceName)
	}

	logExp, err := otlploggrpc.New(
		ctx,
		otlploggrpc.WithEndpoint(cfg.OtelEndpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	globalLogProvider = sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)
	logglobal.SetLoggerProvider(globalLogProvider)

	globalOtelLogger = globalLogProvider.Logger(cfg.ServiceName)

	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)
	globalPropagator = propagator

	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	globalLogger = logger
	initialized = true

	shutdown := func(ctx context.Context) error {
		initMu.Lock()
		defer initMu.Unlock()
		err := shutdownProviders(ctx)
		initialized = false
		return err
	}

	return shutdown, nil
}

func shutdownProviders(ctx context.Context) error {
	var errs []error
	if globalTP != nil {
		errs = append(errs, globalTP.Shutdown(ctx))
		globalTP = nil
	}
	if globalMP != nil {
		errs = append(errs, globalMP.Shutdown(ctx))
		globalMP = nil
	}
	if globalLogProvider != nil {
		errs = append(errs, globalLogProvider.Shutdown(ctx))
		globalLogProvider = nil
	}
	if globalLogger != nil {
		_ = globalLogger.Sync()
		globalLogger = nil
	}
	globalOtelLogger = nil
	globalPropagator = nil
	globalMeter = nil
	return errors.Join(errs...)
}
