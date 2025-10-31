package kafka

import (
	"L0_project/internal/cache"
	"L0_project/internal/config"
	"L0_project/internal/database"
	"L0_project/internal/metrics"
	"L0_project/internal/model"
	"L0_project/internal/validator"
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Consumer читает и обрабатывает сообщения из Kafka.
type Consumer struct {
	reader     *kafka.Reader
	dlqWriter  *kafka.Writer // Продюсер для отправки "битых" сообщений в DLQ
	storage    database.Storage
	cache      cache.Cache
	tracer     trace.Tracer // Для трассировки
	maxRetries int          // Количество попыток для временных ошибок БД
}

// NewConsumer создает новый экземпляр Consumer.
func NewConsumer(cfg config.KafkaConfig, storage database.Storage, cache cache.Cache) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		GroupID:  cfg.GroupID,
		Topic:    cfg.Topic,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
		// Коммиты будут выполняться вручную после успешной обработки.
	})

	// Продюсер для DLQ
	dlqWriter := &kafka.Writer{
		Addr:     kafka.TCP(cfg.Brokers...),
		Topic:    cfg.DLQTopic,
		Balancer: &kafka.LeastBytes{},
	}

	return &Consumer{
		reader:     reader,
		dlqWriter:  dlqWriter,
		storage:    storage,
		cache:      cache,
		tracer:     otel.Tracer("kafka-consumer"),
		maxRetries: 3, // 3 попытки на сохранение в БД
	}
}

// Run запускает цикл чтения сообщений из Kafka.
func (c *Consumer) Run(ctx context.Context) {
	log.Println("Kafka-консюмер запущен...")
	defer func() {
		if err := c.reader.Close(); err != nil {
			log.Printf("Ошибка закрытия Kafka-ридера: %v", err)
		}
		if err := c.dlqWriter.Close(); err != nil {
			log.Printf("Ошибка закрытия Kafka (DLQ) writer: %v", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Println("Kafka-консюмер останавливается.")
			return
		default:
			// FetchMessage используется для ручного контроля коммитов
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				log.Printf("Ошибка чтения сообщения из Kafka: %v", err)
				continue
			}

			// Обрабатываем сообщение
			procErr := c.processMessage(ctx, msg)

			if procErr != nil {
				// Ошибка = нужна повторная обработка.
				// Мы НЕ коммитим сообщение, Kafka доставит его повторно.
				log.Printf("Ошибка обработки сообщения (UID: %s): %v. Не коммитим, ждем retry.", string(msg.Key), procErr)
			} else {
				// nil = обработка успешна (в т.ч. уход в DLQ).
				// Коммитим, чтобы Kafka не присылала его снова.
				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					log.Printf("Ошибка коммита сообщения: %v", err)
				}
			}
		}
	}
}

// processMessage выполняет десериализацию, валидацию, сохранение и кэширование заказа.
// Возвращает error, если нужен Kafka-retry (например, БД временно недоступна).
// Возвращает nil, если обработка успешна или сообщение ушло в DLQ (не нужно ретраить).
func (c *Consumer) processMessage(ctx context.Context, msg kafka.Message) error {
	ctx, span := c.tracer.Start(ctx, "Consumer.processMessage")
	defer span.End()

	var order model.Order
	if err := json.Unmarshal(msg.Value, &order); err != nil {
		log.Printf("Невалидное JSON-сообщение, отправка в DLQ: %v", err)
		c.sendToDLQ(ctx, msg, "json_unmarshal_error", err)
		metrics.KafkaMessagesProcessed.WithLabelValues("dlq_validation").Inc()
		return nil // Коммитим (не ретраим "битый" JSON)
	}

	// Валидация данных
	if err := validator.ValidateStruct(&order); err != nil {
		log.Printf("Ошибка валидации для UID %s, отправка в DLQ: %v", order.OrderUID, err)
		c.sendToDLQ(ctx, msg, "validation_error", err)
		metrics.KafkaMessagesProcessed.WithLabelValues("dlq_validation").Inc()
		return nil // Коммитим (не ретраим невалидные данные)
	}

	// Сохранение в БД с внутренним Retry-циклом
	var dbErr error
	for i := 0; i < c.maxRetries; i++ {
		dbErr = c.storage.SaveOrder(ctx, &order)
		if dbErr == nil {
			break // Успешно
		}
		metrics.DBErrors.WithLabelValues("save_order").Inc()
		log.Printf("Ошибка сохранения в БД (попытка %d/%d): %v", i+1, c.maxRetries, dbErr)
		time.Sleep(time.Second * time.Duration(i+1)) // Простой backoff
	}

	// Если после всех попыток ошибка осталась
	if dbErr != nil {
		log.Printf("Не удалось сохранить заказ %s после %d попыток, отправка в DLQ.", order.OrderUID, c.maxRetries)
		c.sendToDLQ(ctx, msg, "db_save_error", dbErr)
		metrics.KafkaMessagesProcessed.WithLabelValues("dlq_db_error").Inc()
		return nil // Коммитим (не ретраим, т.к. исчерпали попытки)
	}

	log.Printf("Заказ %s успешно сохранен в БД.", order.OrderUID)

	// Кэшируем указатель на копию
	orderCopy := order
	c.cache.Set(ctx, order.OrderUID, &orderCopy) // Передаем контекст
	log.Printf("Заказ %s успешно сохранен в кэш.", order.OrderUID)
	metrics.KafkaMessagesProcessed.WithLabelValues("success").Inc()

	return nil
}

// sendToDLQ отправляет "битое" сообщение в DLQ топик.
func (c *Consumer) sendToDLQ(ctx context.Context, originalMsg kafka.Message, reason string, procErr error) {
	_, span := c.tracer.Start(ctx, "Consumer.sendToDLQ")
	defer span.End()

	// Отправляем сообщение в DLQ с доп. заголовками об ошибке
	err := c.dlqWriter.WriteMessages(ctx, kafka.Message{
		Key:   originalMsg.Key,
		Value: originalMsg.Value,
		Headers: []kafka.Header{
			{Key: "X-Original-Topic", Value: []byte(originalMsg.Topic)},
			{Key: "X-Error-Reason", Value: []byte(reason)},
			{Key: "X-Error-Details", Value: []byte(procErr.Error())},
		},
	})

	if err != nil {
		log.Printf("КРИТИЧНО: Не удалось отправить сообщение %s в DLQ: %v", string(originalMsg.Key), err)
		metrics.KafkaMessagesProcessed.WithLabelValues("dlq_failed_write").Inc()
	} else {
		log.Printf("Сообщение %s отправлено в DLQ (Причина: %s)", string(originalMsg.Key), reason)
	}
}
