package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// @sk-task 61-observability#T1.2: InitProvider initializes OTel TracerProvider and MeterProvider (AC-001, AC-006, AC-007)
func InitProvider(ctx context.Context, endpoint, serviceName, environment string, samplingRatio float64, log *slog.Logger) (func(context.Context) error, error) {
	if endpoint == "" {
		log.WarnContext(ctx, "otel endpoint is empty, tracing and metrics disabled")
		otel.SetTracerProvider(noopTracerProvider())
		otel.SetMeterProvider(noopMeterProvider())
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		return func(_ context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.DeploymentEnvironment(environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		log.WarnContext(ctx, "failed to create trace exporter, tracing disabled", slog.String("error", err.Error()))
		return noopShutdown(), nil
	}

	tpOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter,
			sdktrace.WithBatchTimeout(5*time.Second),
		),
	}

	if samplingRatio < 1.0 {
		tpOpts = append(tpOpts, sdktrace.WithSampler(
			sdktrace.ParentBased(sdktrace.TraceIDRatioBased(samplingRatio)),
		))
	} else {
		tpOpts = append(tpOpts, sdktrace.WithSampler(
			sdktrace.ParentBased(sdktrace.AlwaysSample()),
		))
	}

	tp := sdktrace.NewTracerProvider(tpOpts...)
	otel.SetTracerProvider(tp)

	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		log.WarnContext(ctx, "failed to create metric exporter, metrics export disabled", slog.String("error", err.Error()))
		return tpShutdown(tp), nil
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter,
			sdkmetric.WithInterval(10*time.Second),
		)),
	)
	otel.SetMeterProvider(mp)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return func(ctx context.Context) error {
		var errs []error
		if err := tp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("trace provider shutdown: %w", err))
		}
		if err := mp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
		}
		return errors.Join(errs...)
	}, nil
}

func noopTracerProvider() *sdktrace.TracerProvider {
	return sdktrace.NewTracerProvider()
}

func noopMeterProvider() *sdkmetric.MeterProvider {
	return sdkmetric.NewMeterProvider()
}

func noopShutdown() func(context.Context) error {
	return func(_ context.Context) error { return nil }
}

func tpShutdown(tp *sdktrace.TracerProvider) func(context.Context) error {
	return func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}
}
