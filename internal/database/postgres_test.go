package database

import (
	"L0_project/internal/model"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
)

// helperTestOrder - заказ для тестов
var helperTestOrder = &model.Order{
	OrderUID:    "test-uid-123",
	TrackNumber: "track-123",
	Entry:       "WBIL",
	Delivery: model.Delivery{
		Name: "Test", Phone: "123", Zip: "123", City: "Test", Address: "Test", Region: "Test", Email: "test@test.com",
	},
	Payment: model.Payment{
		Transaction: "test-uid-123", Currency: "USD", Provider: "test", Amount: 100, PaymentDt: 12345, Bank: "test", DeliveryCost: 10, GoodsTotal: 90,
	},
	Items: []model.Item{
		{ChrtID: 1, TrackNumber: "track-123", Price: 100, Rid: "rid-1", Name: "Item 1", Sale: 10, Size: "0", TotalPrice: 90, NmID: 123, Brand: "Test", Status: 202},
	},
	Locale: "en", CustomerID: "test-cust", DeliveryService: "test-ds", Shardkey: "1", SmID: 1, DateCreated: time.Now(), OofShard: "1",
}

// setupStorageWithMock настраивает postgresStorage с моком sqlx.DB
func setupStorageWithMock(t *testing.T) (Storage, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("не удалось создать sqlmock: %v", err)
	}
	sqlxDB := sqlx.NewDb(db, "postgres")

	storage := &postgresStorage{
		db:     sqlxDB,
		tracer: otel.Tracer("postgres-storage-test"),
	}
	return storage, mock
}

func TestPostgresStorage_Close(t *testing.T) {
	storage, mock := setupStorageWithMock(t)

	mock.ExpectClose()

	err := storage.Close()
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStorage_Close_Error(t *testing.T) {
	storage, mock := setupStorageWithMock(t)
	mockErr := errors.New("close error")

	mock.ExpectClose().WillReturnError(mockErr)

	err := storage.Close()
	assert.Error(t, err)
	assert.Equal(t, mockErr, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStorage_SaveOrder_Success(t *testing.T) {
	storage, mock := setupStorageWithMock(t)
	ctx := context.Background()
	order := helperTestOrder

	mock.ExpectBegin()

	// 1. Delivery Insert
	mock.ExpectQuery(`INSERT INTO deliveries`).
		WithArgs(order.Delivery.Name, order.Delivery.Phone, order.Delivery.Zip, order.Delivery.City, order.Delivery.Address, order.Delivery.Region, order.Delivery.Email).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	// 2. Payment Insert
	mock.ExpectQuery(`INSERT INTO payments`).
		WithArgs(order.Payment.Transaction, order.Payment.RequestID, order.Payment.Currency, order.Payment.Provider, order.Payment.Amount, order.Payment.PaymentDt, order.Payment.Bank, order.Payment.DeliveryCost, order.Payment.GoodsTotal, order.Payment.CustomFee).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	// 3. Order Insert
	mock.ExpectExec(`INSERT INTO orders`).
		WithArgs(order.OrderUID, order.TrackNumber, order.Entry, 1, 1, order.Locale, order.InternalSignature, order.CustomerID, order.DeliveryService, order.Shardkey, order.SmID, order.DateCreated, order.OofShard).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 4. Item Insert
	item := order.Items[0]
	mock.ExpectExec(`INSERT INTO items`).
		WithArgs(order.OrderUID, item.ChrtID, item.TrackNumber, item.Price, item.Rid, item.Name, item.Sale, item.Size, item.TotalPrice, item.NmID, item.Brand, item.Status).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	err := storage.SaveOrder(ctx, &order)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStorage_SaveOrder_BeginError(t *testing.T) {
	storage, mock := setupStorageWithMock(t)
	ctx := context.Background()
	mockErr := errors.New("begin error")

	mock.ExpectBegin().WillReturnError(mockErr)

	err := storage.SaveOrder(ctx, &helperTestOrder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ошибка начала транзакции")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStorage_SaveOrder_DeliveryError_Rollback(t *testing.T) {
	storage, mock := setupStorageWithMock(t)
	ctx := context.Background()
	mockErr := errors.New("delivery insert error")

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO deliveries`).WillReturnError(mockErr)
	mock.ExpectRollback() // Ожидаем откат

	err := storage.SaveOrder(ctx, &helperTestOrder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ошибка сохранения доставки")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStorage_SaveOrder_CommitError(t *testing.T) {
	storage, mock := setupStorageWithMock(t)
	ctx := context.Background()
	order := helperTestOrder
	mockErr := errors.New("commit error")

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO deliveries`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`INSERT INTO payments`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectExec(`INSERT INTO orders`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO items`).WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit().WillReturnError(mockErr)
	mock.ExpectRollback() // Ожидаем откат (т.к. defer func() сработает на ошибку)

	err := storage.SaveOrder(ctx, &order)
	assert.Error(t, err)
	assert.Equal(t, mockErr, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStorage_GetOrderByUID_Success(t *testing.T) {
	storage, mock := setupStorageWithMock(t)
	ctx := context.Background()
	order := helperTestOrder
	uid := order.OrderUID

	// 1. Ожидаем запрос заказа (JOIN с delivery и payment)
	orderRows := sqlmock.NewRows([]string{
		"order_uid", "track_number", "entry", "locale", "internal_signature", "customer_id", "delivery_service", "shardkey", "sm_id", "date_created", "oof_shard",
		"delivery.name", "delivery.phone", "delivery.zip", "delivery.city", "delivery.address", "delivery.region", "delivery.email",
		"payment.transaction", "payment.request_id", "payment.currency", "payment.provider", "payment.amount", "payment.payment_dt", "payment.bank", "payment.delivery_cost", "payment.goods_total", "payment.custom_fee",
	}).AddRow(
		order.OrderUID, order.TrackNumber, order.Entry, order.Locale, order.InternalSignature, order.CustomerID, order.DeliveryService, order.Shardkey, order.SmID, order.DateCreated, order.OofShard,
		order.Delivery.Name, order.Delivery.Phone, order.Delivery.Zip, order.Delivery.City, order.Delivery.Address, order.Delivery.Region, order.Delivery.Email,
		order.Payment.Transaction, order.Payment.RequestID, order.Payment.Currency, order.Payment.Provider, order.Payment.Amount, order.Payment.PaymentDt, order.Payment.Bank, order.Payment.DeliveryCost, order.Payment.GoodsTotal, order.Payment.CustomFee,
	)

	mock.ExpectQuery(`SELECT o.order_uid, o.track_number, o.entry`).WithArgs(uid).WillReturnRows(orderRows)

	// 2. Ожидаем запрос товаров
	item := order.Items[0]
	itemRows := sqlmock.NewRows([]string{
		"id", "chrt_id", "track_number", "price", "rid", "name", "sale", "size", "total_price", "nm_id", "brand", "status", "order_uid",
	}).AddRow(
		1, item.ChrtID, item.TrackNumber, item.Price, item.Rid, item.Name, item.Sale, item.Size, item.TotalPrice, item.NmID, item.Brand, item.Status, order.OrderUID,
	)

	mock.ExpectQuery(`SELECT \* FROM items WHERE order_uid`).WithArgs(uid).WillReturnRows(itemRows)

	resultOrder, err := storage.GetOrderByUID(ctx, uid)
	assert.NoError(t, err)
	assert.NotNil(t, resultOrder)
	assert.Equal(t, order.OrderUID, resultOrder.OrderUID)
	assert.Equal(t, order.Delivery.Name, resultOrder.Delivery.Name)
	assert.Equal(t, order.Payment.Transaction, resultOrder.Payment.Transaction)
	assert.Len(t, resultOrder.Items, 1)
	assert.Equal(t, order.Items[0].Name, resultOrder.Items[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStorage_GetOrderByUID_OrderNotFound(t *testing.T) {
	storage, mock := setupStorageWithMock(t)
	ctx := context.Background()
	uid := "not-found-uid"
	mockErr := fmt.Errorf("sql: no rows in result set")

	// 1. Ожидаем запрос заказа (JOIN), который вернет ошибку
	mock.ExpectQuery(`SELECT o.order_uid, o.track_number, o.entry`).
		WithArgs(uid).
		WillReturnError(mockErr)

	// Запрос товаров не должен быть вызван
	mock.ExpectQuery(`SELECT \* FROM items WHERE order_uid`).Times(0)

	resultOrder, err := storage.GetOrderByUID(ctx, uid)
	assert.Error(t, err)
	assert.Nil(t, resultOrder)
	assert.Contains(t, err.Error(), "не удалось получить заказ")
	assert.NoError(t, mock.ExpectationsWereMet())
}
