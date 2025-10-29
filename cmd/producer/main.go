package main

import (
	"L0_project/internal/model"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"io/ioutil"
	"log"
	"math/rand"
	"time"
)

// Producer отвечает за генерацию и отправку сообщений в Kafka.
type Producer struct {
	writer    *kafka.Writer
	baseOrder model.Order
}

// NewProducer создает и настраивает новый экземпляр продюсера.
func NewProducer(brokers []string, topic, modelPath string) (*Producer, error) {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}

	byteValue, err := ioutil.ReadFile(modelPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения файла модели: %w", err)
	}

	var baseOrder model.Order
	if err := json.Unmarshal(byteValue, &baseOrder); err != nil {
		return nil, fmt.Errorf("ошибка парсинга файла модели: %w", err)
	}

	return &Producer{writer: writer, baseOrder: baseOrder}, nil
}

// generateOrder создает новый заказ на основе шаблона с уникальными данными.
func (p *Producer) generateOrder() model.Order {
	newOrder := p.baseOrder
	newOrder.OrderUID = uuid.New().String()
	newOrder.TrackNumber = fmt.Sprintf("WBILM%d", 1000000+rand.Intn(9000000))
	newOrder.DateCreated = time.Now()
	newOrder.Payment.Transaction = newOrder.OrderUID

	for i := range newOrder.Items {
		newOrder.Items[i].TrackNumber = newOrder.TrackNumber
	}
	return newOrder
}

// Run запускает бесконечный цикл отправки сообщений.
func (p *Producer) Run(ctx context.Context, interval time.Duration) {
	log.Println("Продюсер запущен. Нажмите CTRL+C для остановки.")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Продюсер останавливается.")
			return
		case <-ticker.C:
			order := p.generateOrder()
			orderBytes, err := json.Marshal(order)
			if err != nil {
				log.Printf("Ошибка сериализации заказа: %v", err)
				continue
			}

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
	rand.Seed(time.Now().UnixNano())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	producer, err := NewProducer([]string{"localhost:9092"}, "orders", "./model.json")
	if err != nil {
		log.Fatalf("Не удалось создать продюсер: %v", err)
	}
	defer producer.Close()

	producer.Run(ctx, 2*time.Second)
}
