package cache

import (
	"L0_project/internal/database"
	"L0_project/internal/metrics"
	"container/list"
	"context"
	"log"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

//go:generate mockgen -source=lru.go -destination=./mocks/cache_mock.go -package=mocks Cache

// Cache определяет интерфейс для кэширования.
// Контекст добавлен для поддержки сквозной трассировки.
type Cache interface {
	Set(ctx context.Context, key string, value interface{})
	Get(ctx context.Context, key string) (interface{}, bool)
}

// lruCache реализует LRU (Least Recently Used) кэш.
type lruCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*list.Element
	queue    *list.List
	tracer   trace.Tracer // Для трассировки
}

type cacheItem struct {
	key   string
	value interface{}
}

// NewLRUCache создает новый LRU-кэш с заданной емкостью.
func NewLRUCache(capacity int) Cache {
	return &lruCache{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		queue:    list.New(),
		tracer:   otel.Tracer("lru-cache"), // Инициализация трейсера
	}
}

func (c *lruCache) Set(ctx context.Context, key string, value interface{}) {
	// Создаем span для трассировки
	_, span := c.tracer.Start(ctx, "Cache.Set")
	defer span.End()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.capacity <= 0 {
		return
	}

	if element, exists := c.items[key]; exists {
		c.queue.MoveToFront(element)
		element.Value.(*cacheItem).value = value
		return
	}

	if c.queue.Len() >= c.capacity && c.capacity > 0 {
		c.removeOldest()
	}

	item := &cacheItem{key: key, value: value}
	element := c.queue.PushFront(item)
	c.items[key] = element

	// Обновляем метрику размера кэша
	metrics.CacheSize.Set(float64(c.queue.Len()))
}

func (c *lruCache) Get(ctx context.Context, key string) (interface{}, bool) {
	// Создаем span для трассировки
	_, span := c.tracer.Start(ctx, "Cache.Get")
	defer span.End()

	c.mu.Lock()
	defer c.mu.Unlock()

	if element, exists := c.items[key]; exists {
		c.queue.MoveToFront(element)
		return element.Value.(*cacheItem).value, true
	}

	return nil, false
}

// removeOldest удаляет самый старый элемент (внутренняя функция, мьютекс уже захвачен).
func (c *lruCache) removeOldest() {
	element := c.queue.Back()
	if element != nil {
		item := c.queue.Remove(element).(*cacheItem)
		delete(c.items, item.key)

		// Обновляем метрики
		metrics.CacheEvictions.Inc()
		metrics.CacheSize.Set(float64(c.queue.Len()))
	}
}

// WarmUp загружает данные из БД в кэш.
func WarmUp(ctx context.Context, storage database.Storage, cache Cache) error {
	log.Println("Выполняется прогрев кэша...")
	orders, err := storage.GetAllOrders(ctx)
	if err != nil {
		return err
	}

	for _, order := range orders {
		orderCopy := order
		// Передаем контекст
		cache.Set(ctx, order.OrderUID, &orderCopy)
	}

	log.Printf("Кэш прогрет. Загружено %d заказов.", len(orders))
	return nil
}
