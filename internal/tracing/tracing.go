package tracing

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// newJaegerExporter создает экспортер, который отправляет трейсы в Jaeger.
func newJaegerExporter(url string) (sdktrace.SpanExporter, error) {
	return jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
}

// InitTracerProvider настраивает и регистрирует OpenTelemetry-провайдер.
func InitTracerProvider(serviceName string) func() {
	jaegerURL := "http://jaeger:14268/api/traces"
	exporter, err := newJaegerExporter(jaegerURL)
	if err != nil {
		log.Fatalf("Ошибка создания Jaeger-экспортера: %v", err)
	}

	// Ресурс (описание сервиса)
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		serviceName(serviceName), // Имя сервиса
	)

	// Провайдер трассировки
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(1.0)),
	)

	// Регистрируем глобальный провайдер
	otel.SetTracerProvider(tp)

	// Устанавливаем W3C Trace Context в качестве глобального propagator'а
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	log.Println("OpenTelemetry (Jaeger) инициализирован.")

	// Возвращаем функцию shutdown
	return func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Ошибка остановки TracerProvider: %v", err)
		}
	}
}
