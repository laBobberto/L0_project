package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLRUCache_SetAndGet(t *testing.T) {
	cache := NewLRUCache(2)
	assertions := assert.New(t)
	ctx := context.Background()

	// 1. Добавить первый элемент
	cache.Set(ctx, "key1", "value1")
	val, found := cache.Get(ctx, "key1")
	assertions.True(found)
	assertions.Equal("value1", val)

	// 2. Добавить второй элемент
	cache.Set(ctx, "key2", "value2")
	val, found = cache.Get(ctx, "key2")
	assertions.True(found)
	assertions.Equal("value2", val)

	// 3. Проверить, что оба на месте
	val, found = cache.Get(ctx, "key1")
	assertions.True(found)
	assertions.Equal("value1", val)
}

func TestLRUCache_Eviction(t *testing.T) {
	cache := NewLRUCache(2)
	assertions := assert.New(t)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1")
	cache.Set(ctx, "key2", "value2")

	// 4. Добавить третий элемент, "key1" (самый старый) должен вытесниться
	cache.Set(ctx, "key3", "value3")

	// "key1" должен быть удален
	_, found := cache.Get(ctx, "key1")
	assertions.False(found, "key1 should be evicted")

	// "key2" и "key3" должны быть на месте
	val, found := cache.Get(ctx, "key2")
	assertions.True(found)
	assertions.Equal("value2", val)

	val, found = cache.Get(ctx, "key3")
	assertions.True(found)
	assertions.Equal("value3", val)
}

func TestLRUCache_UsageUpdatesOrder(t *testing.T) {
	cache := NewLRUCache(2)
	assertions := assert.New(t)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1")
	cache.Set(ctx, "key2", "value2") // "key1" - старый, "key2" - новый

	// 1. Используем "key1", он должен стать самым новым
	cache.Get(ctx, "key1")

	// 2. Добавляем "key3". Теперь "key2" (как самый старый) должен вытесниться
	cache.Set(ctx, "key3", "value3")

	// "key2" должен быть удален
	_, found := cache.Get(ctx, "key2")
	assertions.False(found, "key2 should be evicted")

	// "key1" и "key3" на месте
	_, found = cache.Get(ctx, "key1")
	assertions.True(found)
	_, found = cache.Get(ctx, "key3")
	assertions.True(found)
}

func TestLRUCache_UpdateValue(t *testing.T) {
	cache := NewLRUCache(2)
	assertions := assert.New(t)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1")
	val, found := cache.Get(ctx, "key1")
	assertions.True(found)
	assertions.Equal("value1", val)

	// Обновляем значение
	cache.Set(ctx, "key1", "value_new")
	val, found = cache.Get(ctx, "key1")
	assertions.True(found)
	assertions.Equal("value_new", val)
}

func TestLRUCache_ZeroCapacity(t *testing.T) {
	// Кэш с 0 емкостью не должен ничего хранить
	cache := NewLRUCache(0)
	assertions := assert.New(t)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1")
	_, found := cache.Get(ctx, "key1")
	assertions.False(found)
}
