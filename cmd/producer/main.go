package main

import (
	"L0_project/internal/generator"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// Producer отвечает только за отправку сообщений в Kafka.
type Producer struct {
	writer *kafka.Writer
}

// NewProducer создает и настраивает новый экземпляр продюсера.
func NewProducer(brokers []string, topic string) (*Producer, error) {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	// Логика чтения model.json полностью удалена.
	return &Producer{writer: writer}, nil
}

// generateOrder() удален. Логика переехала в internal/generator

// Run запускает бесконечный цикл отправки сообщений.
func (p *Producer) Run(ctx context.Context, interval time.Duration) {
	log.Println("Продюсер запущен (использует generator). Нажмите CTRL+C для остановки.")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Продюсер останавливается.")
			return
		case <-ticker.C:
			// 1. Получаем готовый заказ из пакета generator
			order := generator.NewOrder()

			// 2. Сериализуем его
			orderBytes, err := json.Marshal(order)
			if err != nil {
				log.Printf("Ошибка сериализации заказа: %v", err)
				continue
			}

			// 3. Отправляем в Kafka
			err = p.writer.WriteMessages(ctx, kafka.Message{
				Key:   []byte(order.OrderUID),
				Value: orderBytes,
			})

			if err != nil {
				log.Printf("Ошибка отправки сообщения: %v", err)
			} else {
				fmt.Printf("Отправлен заказ с UID: %s\n", order.OrderUID)
			}
		}
	}
}

func (p *Producer) Close() {
	if err := p.writer.Close(); err != nil {
		log.Printf("Ошибка закрытия Kafka writer: %v", err)
	}
}

func main() {
	// rand.Seed() не нужен, gofakeit инициализируется самостоятельно.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Вызов NewProducer теперь чистый, без model.json
	producer, err := NewProducer([]string{"localhost:9092"}, "orders")
	if err != nil {
		log.Fatalf("Не удалось создать продюсер: %v", err)
	}
	defer producer.Close()

	producer.Run(ctx, 3*time.Second) // Отправляем каждые 3 секунды
}
