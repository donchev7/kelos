package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestInjectExtractContextAnnotations(t *testing.T) {
	shutdown, err := Init(context.Background(), "test-service")
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}
	defer func() {
		_ = shutdown(context.Background())
	}()

	ctx, span := Tracer("test").Start(context.Background(), "parent")
	defer span.End()

	annotations := map[string]string{}
	InjectContextToAnnotations(ctx, annotations)

	if annotations[AnnotationTraceParent] == "" {
		t.Fatal("expected traceparent annotation to be set")
	}

	extracted := ExtractContextFromAnnotations(context.Background(), annotations)
	got := trace.SpanContextFromContext(extracted)
	want := trace.SpanContextFromContext(ctx)
	if !got.IsValid() {
		t.Fatal("expected extracted span context to be valid")
	}
	if got.TraceID() != want.TraceID() {
		t.Fatalf("trace ID mismatch: got %s want %s", got.TraceID(), want.TraceID())
	}
}

func TestTraceAnnotationsFiltersOnlyTraceKeys(t *testing.T) {
	got := TraceAnnotations(map[string]string{
		AnnotationTraceParent: "00-11111111111111111111111111111111-2222222222222222-01",
		AnnotationTraceState:  "vendor=value",
		AnnotationBaggage:     "tenant=qa",
		"other":               "ignored",
	})

	if len(got) != 3 {
		t.Fatalf("expected 3 trace annotations, got %d: %v", len(got), got)
	}
	if got["other"] != "" {
		t.Fatalf("expected non-trace annotation to be filtered, got %q", got["other"])
	}
}

func TestHashID(t *testing.T) {
	got := HashID("C123")
	if got == "" || got == "C123" {
		t.Fatalf("expected hashed ID, got %q", got)
	}
	if got != HashID("C123") {
		t.Fatal("expected HashID to be stable")
	}
}
