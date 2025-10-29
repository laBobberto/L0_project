package cache

import (
	"L0_project/internal/database"
	"container/list"
	"context"
	"log"
	"sync"
)

// Cache определяет интерфейс для кэширования.
type Cache interface {
	Set(key string, value interface{})
	Get(key string) (interface{}, bool)
}

// lruCache реализует LRU (Least Recently Used) кэш.
type lruCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*list.Element
	queue    *list.List
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
	}
}

func (c *lruCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

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
}

func (c *lruCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, exists := c.items[key]; exists {
		c.queue.MoveToFront(element)
		return element.Value.(*cacheItem).value, true
	}

	return nil, false
}

func (c *lruCache) removeOldest() {
	element := c.queue.Back()
	if element != nil {
		item := c.queue.Remove(element).(*cacheItem)
		delete(c.items, item.key)
	}
}

// WarmUp загружает данные из БД в кэш.
func WarmUp(ctx context.Context, storage *database.Storage, cache Cache) error {
	log.Println("Выполняется прогрев кэша...")
	orders, err := storage.GetAllOrders(ctx)
	if err != nil {
		return err
	}

	for _, order := range orders {
		orderCopy := order // Копируем, чтобы избежать проблем с указателями
		cache.Set(order.OrderUID, &orderCopy)
	}

	log.Printf("Кэш прогрет. Загружено %d заказов.", len(orders))
	return nil
}
