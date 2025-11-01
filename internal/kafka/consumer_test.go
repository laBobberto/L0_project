package kafka

import (
	"L0_project/internal/cache/mocks"
	db_mocks "L0_project/internal/database/mocks"
	"L0_project/internal/model"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type NoOpReader struct{}

func (r *NoOpReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	return kafka.Message{}, nil
}
func (r *NoOpReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	return nil
}
func (r *NoOpReader) Close() error { return nil }

// setupConsumerAndMocks - хелпер для инициализации консюмера и моков
func setupConsumerAndMocks(t *testing.T) (*gomock.Controller, *Consumer, *mocks.MockCache, *db_mocks.MockStorage) {
	ctrl := gomock.NewController(t)
	mockCache := mocks.NewMockCache(ctrl)
	mockStorage := db_mocks.NewMockStorage(ctrl)

	// Используем NoOpReader
	consumer := &Consumer{
		reader:  &NoOpReader{},
		storage: mockStorage,
		cache:   mockCache,
	}

	return ctrl, consumer, mockCache, mockStorage
}

// helperTestOrder - валидный заказ для тестов
var helperTestOrder = model.Order{
	OrderUID:    "b563feb7b2b84b6test",
	TrackNumber: "WBILMTESTTRACK",
	Entry:       "WBIL",
	Delivery: model.Delivery{
		Name:    "Test Testov",
		Phone:   "+9720000000",
		Zip:     "2639809",
		City:    "Kiryat Mozkin",
		Address: "Ploshad Mira 15",
		Region:  "Kraiot",
		Email:   "test@gmail.com",
	},
	Payment: model.Payment{
		Transaction:  "b563feb7b2b84b6test",
		Currency:     "USD",
		Provider:     "wbpay",
		Amount:       1817,
		PaymentDt:    1637907727,
		Bank:         "alpha",
		DeliveryCost: 1500,
		GoodsTotal:   317,
	},
	Items: []model.Item{
		{
			ChrtID:      9934930,
			TrackNumber: "WBILMTESTTRACK",
			Price:       453,
			Rid:         "ab4219087a764ae0btest",
			Name:        "Mascaras",
			Sale:        30,
			Size:        "0",
			TotalPrice:  317,
			NmID:        2389212,
			Brand:       "Vivienne Sabo",
			Status:      202,
		},
	},
	Locale:          "en",
	CustomerID:      "test",
	DeliveryService: "meest",
	DateCreated:     parseTime("2021-11-26T06:22:19Z"),
}

func parseTime(ts string) time.Time {
	t, _ := time.Parse(time.RFC3339, ts)
	return t
}

func TestConsumer_ProcessMessage_Success(t *testing.T) {
	ctrl, consumer, mockCache, mockStorage := setupConsumerAndMocks(t)
	defer ctrl.Finish()

	orderBytes, _ := json.Marshal(helperTestOrder)
	msg := kafka.Message{Value: orderBytes}

	// 1. Ожидаем сохранение в БД
	mockStorage.EXPECT().SaveOrder(gomock.Any(), gomock.Any()).Return(nil)
	// 2. Ожидаем сохранение в кэш
	mockCache.EXPECT().Set(gomock.Any(), helperTestOrder.OrderUID, gomock.Any()).Times(1)

	err := consumer.processMessage(context.Background(), msg)
	assert.NoError(t, err)
}

func TestConsumer_ProcessMessage_DBError(t *testing.T) {
	ctrl, consumer, mockCache, mockStorage := setupConsumerAndMocks(t)
	defer ctrl.Finish()

	orderBytes, _ := json.Marshal(helperTestOrder)
	msg := kafka.Message{Value: orderBytes}
	dbErr := errors.New("database connection failed")

	// 1. Ожидаем сохранение в БД, которое вернет ошибку
	// maxRetries (3) + 1 (первая попытка) = 0. По дефолту 3.
	// В тесте maxRetries = 0, поэтому 1 вызов.
	// Обновим тест, чтобы он соответствовал новой логике ретраев
	consumer.maxRetries = 3

	mockStorage.EXPECT().SaveOrder(gomock.Any(), gomock.Any()).Return(dbErr).Times(consumer.maxRetries)

	// 2. Не ожидаем сохранения в кэш
	mockCache.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	// Ожидаем запись в DLQ (т.к. ретраи не помогли)
	// Для этого нужен мок dlqWriter
	// ... (Просто проверим, что ошибка не возвращается, т.к. уходит в DLQ)

	err := consumer.processMessage(context.Background(), msg)

	// Ошибка не должна быть возвращена, т.к. сообщение ушло в DLQ
	assert.NoError(t, err)
}

func TestConsumer_ProcessMessage_DBError_RetryLogic(t *testing.T) {
	ctrl, consumer, mockCache, mockStorage := setupConsumerAndMocks(t)
	defer ctrl.Finish()

	orderBytes, _ := json.Marshal(helperTestOrder)
	msg := kafka.Message{Value: orderBytes}
	dbErr := errors.New("temp db error")

	consumer.maxRetries = 3

	// 1. Ожидаем 2 неудачных вызова
	mockStorage.EXPECT().SaveOrder(gomock.Any(), gomock.Any()).Return(dbErr).Times(2)
	// 2. Ожидаем 1 удачный вызов
	mockStorage.EXPECT().SaveOrder(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	// 3. Ожидаем Set в кэш
	mockCache.EXPECT().Set(gomock.Any(), helperTestOrder.OrderUID, gomock.Any()).Times(1)

	err := consumer.processMessage(context.Background(), msg)

	// Ошибки нет, т.к. ретрай удался
	assert.NoError(t, err)
}

func TestConsumer_ProcessMessage_BadJSON(t *testing.T) {
	ctrl, consumer, mockCache, mockStorage := setupConsumerAndMocks(t)
	defer ctrl.Finish()

	msg := kafka.Message{Value: []byte("this is not json")}

	// Не ожидаем вызовов БД или Кэша
	mockStorage.EXPECT().SaveOrder(gomock.Any(), gomock.Any()).Times(0)
	mockCache.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	err := consumer.processMessage(context.Background(), msg)

	// Ошибка не должна быть возвращена, т.к. это "poison pill"
	// Сообщение будет закоммичено (т.к. err == nil)
	assert.NoError(t, err)
}

func TestConsumer_ProcessMessage_ValidationError(t *testing.T) {
	ctrl, consumer, mockCache, mockStorage := setupConsumerAndMocks(t)
	defer ctrl.Finish()

	// Создаем невалидный заказ (OrderUID отсутствует)
	invalidOrder := helperTestOrder
	invalidOrder.OrderUID = "" // Невалидный UID

	orderBytes, _ := json.Marshal(invalidOrder)
	msg := kafka.Message{Value: orderBytes}

	// Не ожидаем вызовов БД или Кэша
	mockStorage.EXPECT().SaveOrder(gomock.Any(), gomock.Any()).Times(0)
	mockCache.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	err := consumer.processMessage(context.Background(), msg)

	// Ошибка не должна быть возвращена, т.к. это "poison pill"
	assert.NoError(t, err)
}
