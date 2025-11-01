package api

import (
	"L0_project/internal/cache"
	"L0_project/internal/database"
	"L0_project/internal/metrics"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

// OrderHandler обрабатывает HTTP-запросы, связанные с заказами.
type OrderHandler struct {
	storage database.Storage // Используем интерфейс
	cache   cache.Cache      // Используем интерфейс
}

// NewOrderHandler создает новый экземпляр OrderHandler.
func NewOrderHandler(storage database.Storage, cache cache.Cache) *OrderHandler {
	return &OrderHandler{storage: storage, cache: cache}
}

// GetByUID ищет заказ по UID сначала в кэше, затем в БД.
func (h *OrderHandler) GetByUID(w http.ResponseWriter, r *http.Request) {
	// Метрики и трассировка
	const handlerName = "GetByUID"
	timer := prometheus.NewTimer(metrics.HttpRequestDuration.WithLabelValues(handlerName))
	defer timer.ObserveDuration() // Замеряем длительность запроса

	orderUID := chi.URLParam(r, "orderUID")
	if orderUID == "" {
		respondWithError(w, http.StatusBadRequest, "UID заказа не указан", handlerName)
		return
	}

	// 1. Поиск в кэше. Передаем контекст (r.Context()) для трейсинга.
	if order, found := h.cache.Get(r.Context(), orderUID); found {
		log.Printf("КЭШ ХИТ: %s", orderUID)
		metrics.CacheHits.Inc()
		metrics.HttpRequestsTotal.WithLabelValues(handlerName, "200").Inc()
		respondWithJSON(w, http.StatusOK, order)
		return
	}

	// 2. Поиск в БД
	log.Printf("КЭШ ПРОМАХ: %s. Запрос к БД.", orderUID)
	metrics.CacheMisses.Inc()

	// Передаем контекст (r.Context()) для трейсинга.
	order, err := h.storage.GetOrderByUID(r.Context(), orderUID)
	if err != nil {
		log.Printf("Ошибка получения заказа из БД: %v", err)
		metrics.DBErrors.WithLabelValues("get_order").Inc()
		respondWithError(w, http.StatusNotFound, "Заказ не найден", handlerName)
		return
	}

	// 3. Сохранение в кэш. Передаем контекст.
	h.cache.Set(r.Context(), orderUID, order)
	log.Printf("Заказ %s добавлен в кэш.", orderUID)

	metrics.HttpRequestsTotal.WithLabelValues(handlerName, "200").Inc()
	respondWithJSON(w, http.StatusOK, order)
}

// respondWithJSON вспомогательная функция для отправки JSON-ответов.
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Ошибка сериализации JSON: %v", err)
		http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if _, err := w.Write(response); err != nil {
		log.Printf("Ошибка записи HTTP-ответа: %v", err)
	}
}

// respondWithError вспомогательная функция для отправки ошибок и регистрации метрик.
func respondWithError(w http.ResponseWriter, code int, message string, handlerName string) {
	metrics.HttpRequestsTotal.WithLabelValues(handlerName, strconv.Itoa(code)).Inc()
	http.Error(w, message, code)
}
