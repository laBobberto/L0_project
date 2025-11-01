package database

import (
	"L0_project/internal/metrics"
	"L0_project/internal/model"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

//go:generate mockgen -source=postgres.go -destination=./mocks/storage_mock.go -package=mocks Storage

// Storage определяет интерфейс для работы с хранилищем заказов.
type Storage interface {
	SaveOrder(ctx context.Context, order *model.Order) error
	GetOrderByUID(ctx context.Context, orderUID string) (*model.Order, error)
	GetAllOrders(ctx context.Context) ([]model.Order, error)
	Close() error
}

// postgresStorage обеспечивает взаимодействие с базой данных PostgreSQL.
// Это конкретная реализация интерфейса Storage.
type postgresStorage struct {
	db     *sqlx.DB
	tracer trace.Tracer // Для трассировки
}

// New создает подключение к БД, применяет миграции и возвращает
// экземпляр, реализующий интерфейс Storage.
func New(dbURL, migrationsPath string) (Storage, error) {
	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("не удалось подключиться к БД: %w", err)
	}

	// Запуск миграций
	if err := runMigrations(dbURL, migrationsPath); err != nil {
		return nil, fmt.Errorf("ошибка применения миграций: %w", err)
	}

	return &postgresStorage{
		db:     db,
		tracer: otel.Tracer("postgres-storage"), // Инициализация трейсера
	}, nil
}

// runMigrations выполняет миграции БД до последней версии.
func runMigrations(dbURL, migrationsPath string) error {
	log.Println("Поиск и применение миграций...")

	// Важно: 'file://' префикс
	m, err := migrate.New(fmt.Sprintf("file://%s", migrationsPath), dbURL)
	if err != nil {
		return fmt.Errorf("не удалось создать экземпляр миграции: %w", err)
	}

	// Применяем миграции "вверх"
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("не удалось выполнить миграции: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil {
		return fmt.Errorf("не удалось получить версию миграции: %w", err)
	}

	if dirty {
		log.Printf("БД в 'грязном' состоянии (dirty). Версия: %d. Рекомендуется проверка.", version)
	}

	log.Printf("Миграции успешно применены. Текущая версия БД: %d", version)
	return nil
}

// SaveOrder сохраняет заказ и все связанные с ним данные в одной транзакции.
func (s *postgresStorage) SaveOrder(ctx context.Context, order *model.Order) (err error) {
	// Создаем span для трассировки
	ctx, span := s.tracer.Start(ctx, "DB.SaveOrder")
	defer span.End()

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}

	// Используем defer с функцией, чтобы корректно обработать panic и ошибки
	defer func() {
		if p := recover(); p != nil {
			// Если была паника, откатываем
			_ = tx.Rollback()
			panic(p) // Восстанавливаем панику
		} else if err != nil {
			// Если функция завершилась с ошибкой, откатываем
			// Логгируем ошибку отката, если она не sql.ErrTxDone
			if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
				log.Printf("Ошибка отката транзакции (после ошибки: %v): %v", err, rbErr)
			}
		}
	}()

	deliveryQuery := `INSERT INTO deliveries (name, phone, zip, city, address, region, email) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	var deliveryID int
	// Присваиваем ошибку именованной err
	if err = tx.GetContext(ctx, &deliveryID, deliveryQuery, order.Delivery.Name, order.Delivery.Phone, order.Delivery.Zip, order.Delivery.City, order.Delivery.Address, order.Delivery.Region, order.Delivery.Email); err != nil {
		return fmt.Errorf("ошибка сохранения доставки: %w", err)
	}

	paymentQuery := `INSERT INTO payments (transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`
	var paymentID int
	if err = tx.GetContext(ctx, &paymentID, paymentQuery, order.Payment.Transaction, order.Payment.RequestID, order.Payment.Currency, order.Payment.Provider, order.Payment.Amount, order.Payment.PaymentDt, order.Payment.Bank, order.Payment.DeliveryCost, order.Payment.GoodsTotal, order.Payment.CustomFee); err != nil {
		return fmt.Errorf("ошибка сохранения платежа: %w", err)
	}

	orderQuery := `INSERT INTO orders (order_uid, track_number, entry, delivery_id, payment_id, locale, internal_signature, customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`
	if _, err = tx.ExecContext(ctx, orderQuery, order.OrderUID, order.TrackNumber, order.Entry, deliveryID, paymentID, order.Locale, order.InternalSignature, order.CustomerID, order.DeliveryService, order.Shardkey, order.SmID, order.DateCreated, order.OofShard); err != nil {
		return fmt.Errorf("ошибка сохранения заказа: %w", err)
	}

	for _, item := range order.Items {
		itemQuery := `INSERT INTO items (order_uid, chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
		if _, err = tx.ExecContext(ctx, itemQuery, order.OrderUID, item.ChrtID, item.TrackNumber, item.Price, item.Rid, item.Name, item.Sale, item.Size, item.TotalPrice, item.NmID, item.Brand, item.Status); err != nil {
			return fmt.Errorf("ошибка сохранения товара: %w", err)
		}
	}

	// Если все успешно, коммитим. Ошибка (nil или реальная) будет возвращена.
	err = tx.Commit()
	return err
}

// GetOrderByUID извлекает полный объект заказа по его UID.
func (s *postgresStorage) GetOrderByUID(ctx context.Context, orderUID string) (*model.Order, error) {
	// Создаем span для трассировки
	ctx, span := s.tracer.Start(ctx, "DB.GetOrderByUID")
	defer span.End()

	var order model.Order
	query := `
        SELECT
            o.order_uid, o.track_number, o.entry, o.locale, o.internal_signature, o.customer_id, o.delivery_service,
            o.shardkey, o.sm_id, o.date_created, o.oof_shard,
            d.name "delivery.name", d.phone "delivery.phone", d.zip "delivery.zip", d.city "delivery.city",
            d.address "delivery.address", d.region "delivery.region", d.email "delivery.email",
            p.transaction "payment.transaction", p.request_id "payment.request_id", p.currency "payment.currency",
            p.provider "payment.provider", p.amount "payment.amount", p.payment_dt "payment.payment_dt", p.bank "payment.bank",
            p.delivery_cost "payment.delivery_cost", p.goods_total "payment.goods_total", p.custom_fee "payment.custom_fee"
        FROM orders o
        JOIN deliveries d ON o.delivery_id = d.id
        JOIN payments p ON o.payment_id = p.id
        WHERE o.order_uid = $1`

	if err := s.db.GetContext(ctx, &order, query, orderUID); err != nil {
		metrics.DBErrors.WithLabelValues("get_order").Inc() // Метрика ошибки
		return nil, fmt.Errorf("не удалось получить заказ: %w", err)
	}

	if err := s.db.SelectContext(ctx, &order.Items, `SELECT * FROM items WHERE order_uid = $1`, orderUID); err != nil {
		metrics.DBErrors.WithLabelValues("get_items").Inc() // Метрика ошибки
		return nil, fmt.Errorf("не удалось получить товары для заказа: %w", err)
	}

	return &order, nil
}

// GetAllOrders извлекает все заказы из БД для прогрева кэша.
func (s *postgresStorage) GetAllOrders(ctx context.Context) ([]model.Order, error) {
	// Создаем span для трассировки
	ctx, span := s.tracer.Start(ctx, "DB.GetAllOrders")
	defer span.End()

	// Этот запрос получает все данные одним махом, избегая проблемы N+1.
	query := `
        SELECT
            o.order_uid, o.track_number, o.entry, o.locale, o.internal_signature, o.customer_id, 
            o.delivery_service, o.shardkey, o.sm_id, o.date_created, o.oof_shard,

            d.id "delivery.id", d.name "delivery.name", d.phone "delivery.phone", d.zip "delivery.zip", d.city "delivery.city", d.address "delivery.address", d.region "delivery.region", d.email "delivery.email",
            p.id "payment.id", p.transaction "payment.transaction", p.request_id "payment.request_id", p.currency "payment.currency", p.provider "payment.provider", p.amount "payment.amount", p.payment_dt "payment.payment_dt", p.bank "payment.bank", p.delivery_cost "payment.delivery_cost", p.goods_total "payment.goods_total", p.custom_fee "payment.custom_fee",
            i.id "items.id", i.chrt_id "items.chrt_id", i.track_number "items.track_number", i.price "items.price", i.rid "items.rid", i.name "items.name", i.sale "items.sale", i.size "items.size", i.total_price "items.total_price", i.nm_id "items.nm_id", i.brand "items.brand", i.status "items.status"
        
		FROM orders o
        LEFT JOIN deliveries d ON o.delivery_id = d.id
        LEFT JOIN payments p ON o.payment_id = p.id
        LEFT JOIN items i ON o.order_uid = i.order_uid
        ORDER BY o.date_created DESC`

	type fullOrderRow struct {
		model.Order
		model.Delivery `db:"delivery"`
		model.Payment  `db:"payment"`
		model.Item     `db:"items"`
	}

	var rows []fullOrderRow
	if err := s.db.SelectContext(ctx, &rows, query); err != nil {
		metrics.DBErrors.WithLabelValues("get_all_orders").Inc() // Метрика ошибки
		return nil, fmt.Errorf("ошибка получения всех заказов: %w", err)
	}

	// Группируем товары по заказам.
	ordersMap := make(map[string]*model.Order)
	for _, row := range rows {
		if _, exists := ordersMap[row.Order.OrderUID]; !exists {
			order := row.Order
			order.Delivery = row.Delivery
			order.Payment = row.Payment
			order.Items = []model.Item{}
			ordersMap[order.OrderUID] = &order
		}
		if row.Item.ID > 0 { // Проверяем, что товар существует
			order := ordersMap[row.Order.OrderUID]
			order.Items = append(order.Items, row.Item)
		}
	}

	orders := make([]model.Order, 0, len(ordersMap))
	for _, order := range ordersMap {
		orders = append(orders, *order)
	}

	return orders, nil
}

// Close закрывает соединение с БД.
func (s *postgresStorage) Close() error {
	return s.db.Close()
}
