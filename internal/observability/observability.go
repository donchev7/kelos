package observability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	AnnotationTraceParent = "kelos.dev/traceparent"
	AnnotationTraceState  = "kelos.dev/tracestate"
	AnnotationBaggage     = "kelos.dev/baggage"

	EnvTraceParent = "TRACEPARENT"
	EnvTraceState  = "TRACESTATE"
	EnvBaggage     = "BAGGAGE"

	propagationTraceParent = "traceparent"
	propagationTraceState  = "tracestate"
	propagationBaggage     = "baggage"
)

type ShutdownFunc func(context.Context) error

func Init(ctx context.Context, defaultServiceName string) (ShutdownFunc, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	serviceName := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	if serviceName == "" {
		serviceName = defaultServiceName
	}
	if serviceName == "" {
		serviceName = "kelos"
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithAttributes(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		return nil, err
	}

	processors, err := spanProcessors(ctx)
	if err != nil {
		return nil, err
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
	}
	for _, processor := range processors {
		opts = append(opts, sdktrace.WithSpanProcessor(processor))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)

	return func(ctx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(shutdownCtx)
	}, nil
}

func spanProcessors(ctx context.Context) ([]sdktrace.SpanProcessor, error) {
	exporters := strings.TrimSpace(os.Getenv("OTEL_TRACES_EXPORTER"))
	if exporters == "" || exporters == "none" {
		return nil, nil
	}

	var processors []sdktrace.SpanProcessor
	for _, exporter := range strings.Split(exporters, ",") {
		switch strings.TrimSpace(exporter) {
		case "", "none":
			continue
		case "otlp":
			exp, err := otlptracehttp.New(ctx)
			if err != nil {
				return nil, err
			}
			processors = append(processors, sdktrace.NewBatchSpanProcessor(exp))
		default:
			return nil, errors.New("unsupported OTEL_TRACES_EXPORTER: " + exporter)
		}
	}
	return processors, nil
}

func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func InjectContextToAnnotations(ctx context.Context, annotations map[string]string) {
	if annotations == nil {
		return
	}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if v := carrier.Get(propagationTraceParent); v != "" {
		annotations[AnnotationTraceParent] = v
	}
	if v := carrier.Get(propagationTraceState); v != "" {
		annotations[AnnotationTraceState] = v
	}
	if v := carrier.Get(propagationBaggage); v != "" {
		annotations[AnnotationBaggage] = v
	}
}

func ExtractContextFromAnnotations(ctx context.Context, annotations map[string]string) context.Context {
	if annotations == nil {
		return ctx
	}
	carrier := propagation.MapCarrier{}
	if v := annotations[AnnotationTraceParent]; v != "" {
		carrier.Set(propagationTraceParent, v)
	}
	if v := annotations[AnnotationTraceState]; v != "" {
		carrier.Set(propagationTraceState, v)
	}
	if v := annotations[AnnotationBaggage]; v != "" {
		carrier.Set(propagationBaggage, v)
	}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

func ExtractContextFromEnv(ctx context.Context) context.Context {
	carrier := propagation.MapCarrier{}
	if v := os.Getenv(EnvTraceParent); v != "" {
		carrier.Set(propagationTraceParent, v)
	}
	if v := os.Getenv(EnvTraceState); v != "" {
		carrier.Set(propagationTraceState, v)
	}
	if v := os.Getenv(EnvBaggage); v != "" {
		carrier.Set(propagationBaggage, v)
	}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

func TraceAnnotations(annotations map[string]string) map[string]string {
	out := map[string]string{}
	for _, key := range []string{AnnotationTraceParent, AnnotationTraceState, AnnotationBaggage} {
		if v := annotations[key]; v != "" {
			out[key] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func RecordError(span trace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func HashID(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])[:16]
}
