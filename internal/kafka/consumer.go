package kafka

import (
	"L0_project/internal/cache"
	"L0_project/internal/config"
	"L0_project/internal/database"
	"L0_project/internal/model"
	"L0_project/internal/validator"
	"context"
	"encoding/json"
	"log"

	"github.com/segmentio/kafka-go"
)

// Consumer читает и обрабатывает сообщения из Kafka.
type Consumer struct {
	reader  *kafka.Reader
	storage database.Storage
	cache   cache.Cache
}

// NewConsumer создает новый экземпляр Consumer.
func NewConsumer(cfg config.KafkaConfig, storage database.Storage, cache cache.Cache) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		GroupID:  cfg.GroupID,
		Topic:    cfg.Topic,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})
	return &Consumer{reader: reader, storage: storage, cache: cache}
}

// Run запускает цикл чтения сообщений из Kafka.
func (c *Consumer) Run(ctx context.Context) {
	log.Println("Kafka-консюмер запущен...")
	defer func() {
		if err := c.reader.Close(); err != nil {
			log.Printf("Ошибка закрытия Kafka-ридера: %v", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Println("Kafka-консюмер останавливается.")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				log.Printf("Ошибка чтения сообщения из Kafka: %v", err)
				continue
			}

			if err := c.processMessage(ctx, msg); err != nil {
				log.Printf("Ошибка обработки сообщения (UID: %s): %v", string(msg.Key), err)
			} else {
				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					log.Printf("Ошибка коммита сообщения: %v", err)
				}
			}
		}
	}
}

// processMessage выполняет десериализацию, валидацию, сохранение и кэширование заказа.
func (c *Consumer) processMessage(ctx context.Context, msg kafka.Message) error {
	var order model.Order
	if err := json.Unmarshal(msg.Value, &order); err != nil {
		log.Printf("Невалидное JSON-сообщение, пропускаем: %v", err)
		return nil
	}

	// Валидация данных
	if err := validator.ValidateStruct(&order); err != nil {
		log.Printf("Ошибка валидации данных для UID %s, сообщение пропущено: %v", order.OrderUID, err)
		return nil
	}

	// Сохранение в БД
	if err := c.storage.SaveOrder(ctx, &order); err != nil {
		return err // Ошибка сохранения в БД (проблемы с БД), сообщение будет обработано повторно.
	}
	log.Printf("Заказ %s успешно сохранен в БД.", order.OrderUID)

	// Кэшируем указатель на копию
	orderCopy := order
	c.cache.Set(order.OrderUID, &orderCopy)
	log.Printf("Заказ %s успешно сохранен в кэш.", order.OrderUID)

	return nil
}
