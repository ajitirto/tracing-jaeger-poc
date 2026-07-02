package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "backend-service"

// Inisialisasi OpenTelemetry Tracer
func initTracer() (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint("localhost:4318"),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tp, nil
}

func main() {
	tp, err := initTracer()
	if err != nil {
		log.Fatalf("Gagal inisialisasi tracer: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Gagal menghentikan tracer: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/data", dataHandler)

	// Wrap dengan CORS Middleware
	log.Println("Backend berjalan di http://localhost:8085")
	log.Fatal(http.ListenAndServe(":8085", corsMiddleware(mux)))
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	// Karena di frontend tidak pakai OTel, Golang langsung membuat Root Trace baru di sini
	tracer := otel.GetTracerProvider().Tracer(serviceName)

	// Membuat span utama untuk HTTP Request ini
	ctx, span := tracer.Start(r.Context(), "HTTP GET /api/data")
	defer span.End()

	// Menambahkan informasi tambahan (opsional)
	span.SetAttributes(
		semconv.HTTPMethodKey.String(r.Method),
		semconv.HTTPTargetKey.String(r.URL.Path),
	)

	// Jalankan logic database (ini akan otomatis jadi Child Span)
	processDatabaseLogic(ctx, tracer)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "success", "message": "Halo dari Golang! Tracing Jaeger Sukses."}`))
}

func processDatabaseLogic(ctx context.Context, tracer trace.Tracer) {
	_, span := tracer.Start(ctx, "Query Database SQL")
	defer span.End()

	// Simulasi query memakan waktu 200ms
	time.Sleep(200 * time.Millisecond)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, traceparent")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
