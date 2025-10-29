package api

import (
	"L0_project/internal/cache"
	"L0_project/internal/database"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"log"
	"net/http"
)

// OrderHandler обрабатывает HTTP-запросы, связанные с заказами.
type OrderHandler struct {
	storage *database.Storage
	cache   cache.Cache
}

// NewOrderHandler создает новый экземпляр OrderHandler.
func NewOrderHandler(storage *database.Storage, cache cache.Cache) *OrderHandler {
	return &OrderHandler{storage: storage, cache: cache}
}

// GetByUID ищет заказ по UID сначала в кэше, затем в БД.
func (h *OrderHandler) GetByUID(w http.ResponseWriter, r *http.Request) {
	orderUID := chi.URLParam(r, "orderUID")
	if orderUID == "" {
		http.Error(w, "UID заказа не указан", http.StatusBadRequest)
		return
	}

	// Поиск в кэше
	if order, found := h.cache.Get(orderUID); found {
		log.Printf("КЭШ ХИТ: %s", orderUID)
		respondWithJSON(w, http.StatusOK, order)
		return
	}

	// Поиск в БД
	log.Printf("КЭШ ПРОМАХ: %s. Запрос к БД.", orderUID)
	order, err := h.storage.GetOrderByUID(r.Context(), orderUID)
	if err != nil {
		log.Printf("Ошибка получения заказа из БД: %v", err)
		http.Error(w, "Заказ не найден", http.StatusNotFound)
		return
	}

	// Сохранение в кэш
	h.cache.Set(orderUID, order)
	log.Printf("Заказ %s добавлен в кэш.", orderUID)

	respondWithJSON(w, http.StatusOK, order)
}

// respondWithJSON вспомогательная функция для отправки JSON-ответов.
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
