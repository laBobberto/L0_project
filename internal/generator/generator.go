package generator

import (
	"L0_project/internal/model"
	"fmt"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/google/uuid"
)

// NewOrder создает и возвращает один полностью случайный заказ.
// Эта функция инкапсулирует всю логику генерации тестовых данных.
func NewOrder() model.Order {
	// Инициализируем gofakeit, если это еще не сделано (на всякий случай)
	gofakeit.Seed(0)

	// Один трек-номер на весь заказ
	trackNumber := fmt.Sprintf("WBILM%d", gofakeit.Number(1000000, 9999999))
	goodsTotal := 0

	// 1. Генерируем состав заказа (Items)
	var items []model.Item
	itemCount := gofakeit.Number(1, 4) // От 1 до 4 товаров

	for i := 0; i < itemCount; i++ {
		price := gofakeit.Number(1000, 25000)
		sale := gofakeit.Number(5, 50) // Скидка от 5% до 50%
		totalPrice := (price * (100 - sale)) / 100
		goodsTotal += totalPrice

		item := model.Item{
			ChrtID:      gofakeit.Number(1000000, 9999999),
			TrackNumber: trackNumber, // Используем общий трек-номер
			Price:       price,
			Rid:         uuid.New().String(),
			Name:        gofakeit.ProductName(),
			Sale:        sale,
			Size:        gofakeit.RandomString([]string{"S", "M", "L", "XL", "0"}),
			TotalPrice:  totalPrice,
			NmID:        gofakeit.Number(1000000, 9999999),
			Brand:       gofakeit.Company(),
			Status:      202, // Статус "в пути"
		}
		items = append(items, item)
	}

	// 2. Генерируем данные о доставке (Delivery)
	// Генерируем один адресный объект, чтобы город, штат и zip-код
	// были согласованы друг с другом.
	addr := gofakeit.Address()

	delivery := model.Delivery{
		Name:    gofakeit.Name(),
		Phone:   gofakeit.Phone(), // Генерирует в формате +...
		Zip:     addr.Zip,         // Используем согласованный Zip
		City:    addr.City,        // Используем согласованный City
		Address: addr.Address,     // Используем согласованный Address
		Region:  addr.State,       // Используем согласованный State (Region)
		Email:   gofakeit.Email(),
	}

	// 3. Генерируем данные об оплате (Payment)
	deliveryCost := gofakeit.Number(150, 1000)
	orderUID := uuid.New().String() // UID заказа

	payment := model.Payment{
		Transaction:  orderUID, // Связываем транзакцию с UID заказа
		RequestID:    "",
		Currency:     gofakeit.CurrencyShort(), // "USD", "EUR" и т.д.
		Provider:     gofakeit.RandomString([]string{"wbpay", "click", "paypal"}),
		Amount:       goodsTotal + deliveryCost,                             // Общая сумма
		PaymentDt:    time.Now().Unix() - int64(gofakeit.Number(100, 1000)), // Недавнее прошлое
		Bank:         gofakeit.RandomString([]string{"sber", "alpha", "tinkoff", "vtb"}),
		DeliveryCost: deliveryCost,
		GoodsTotal:   goodsTotal,
		CustomFee:    0,
	}

	// 4. Собираем финальный заказ (Order)
	order := model.Order{
		OrderUID:          orderUID,
		TrackNumber:       trackNumber,
		Entry:             "WBIL",
		Delivery:          delivery,
		Payment:           payment,
		Items:             items,
		Locale:            gofakeit.LanguageAbbreviation(), // "en", "ru"
		InternalSignature: "",
		CustomerID:        gofakeit.Username(),
		DeliveryService:   gofakeit.RandomString([]string{"meest", "dhl", "pony"}),
		Shardkey:          fmt.Sprintf("%d", gofakeit.Number(1, 10)),
		SmID:              gofakeit.Number(1, 100),
		DateCreated:       time.Now().Add(-time.Duration(gofakeit.Number(1, 100)) * time.Minute),
		OofShard:          "1",
	}

	return order
}
