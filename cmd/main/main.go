package main

import (
	"L0_project/internal/api"
	"L0_project/internal/cache"
	"L0_project/internal/config"
	"L0_project/internal/database"
	"L0_project/internal/kafka"
	"L0_project/internal/metrics"
	"L0_project/internal/tracing"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	shutdownTracer := tracing.InitTracerProvider("l0-app")
	defer shutdownTracer()
	cfg := config.Get()

	// Инициализация метрик (Prometheus)
	metrics.Init()

	// Инициализация хранилища
	// Путь изменен на папку с миграциями
	storage, err := database.New(cfg.Postgres.URL, "./internal/database/migrations")
	if err != nil {
		log.Fatalf("Ошибка инициализации хранилища: %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			log.Printf("Ошибка закрытия хранилища: %v", err)
		}
	}()

	// Инициализация кэша
	orderCache := cache.NewLRUCache(cfg.Cache.Size)
	// Используем Background-контекст для прогрева, т.к. он должен завершиться до старта
	if err := cache.WarmUp(context.Background(), storage, orderCache); err != nil {
		log.Printf("Ошибка при прогреве кэша: %v", err)
	}

	// Запуск Kafka Consumer
	ctx, cancel := context.WithCancel(context.Background())
	consumer := kafka.NewConsumer(cfg.Kafka, storage, orderCache)
	go consumer.Run(ctx)

	// Запуск HTTP-сервера
	server := api.NewServer(cfg.HTTP.Port, storage, orderCache)
	go func() {
		if err := server.Run(); err != nil {
			log.Fatalf("Ошибка запуска HTTP-сервера: %v", err)
		}
	}()

	// Ожидание сигнала для корректного завершения работы
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown

	log.Println("Сервис останавливается...")
	cancel() // Отправляем сигнал отмены во все компоненты (Kafka)
	log.Println("Сервис успешно остановлен.")
}
