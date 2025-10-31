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
	handlerName := "GetByUID"
	timer := prometheus.NewTimer(metrics.HttpRequestDuration.WithLabelValues(handlerName))
	defer timer.ObserveDuration() // Замеряем длительность запроса

	orderUID := chi.URLParam(r, "orderUID")
	if orderUID == "" {
		metrics.HttpRequestsTotal.WithLabelValues(handlerName, "400").Inc()
		http.Error(w, "UID заказа не указан", http.StatusBadRequest)
		return
	}

	// Поиск в кэше. Передаем контекст (r.Context()) для трейсинга.
	if order, found := h.cache.Get(r.Context(), orderUID); found {
		log.Printf("КЭШ ХИТ: %s", orderUID)
		metrics.CacheHits.Inc()
		metrics.HttpRequestsTotal.WithLabelValues(handlerName, "200").Inc()
		respondWithJSON(w, http.StatusOK, order)
		return
	}

	// Поиск в БД
	log.Printf("КЭШ ПРОМАХ: %s. Запрос к БД.", orderUID)
	metrics.CacheMisses.Inc()

	// Передаем контекст (r.Context()) для трейсинга.
	order, err := h.storage.GetOrderByUID(r.Context(), orderUID)
	if err != nil {
		log.Printf("Ошибка получения заказа из БД: %v", err)
		metrics.DBErrors.WithLabelValues("get_order").Inc()
		metrics.HttpRequestsTotal.WithLabelValues(handlerName, "404").Inc()
		http.Error(w, "Заказ не найден", http.StatusNotFound)
		return
	}

	// Сохранение в кэш. Передаем контекст.
	h.cache.Set(r.Context(), orderUID, order)
	log.Printf("Заказ %s добавлен в кэш.", orderUID)

	metrics.HttpRequestsTotal.WithLabelValues(handlerName, "200").Inc()
	respondWithJSON(w, http.StatusOK, order)
}

// respondWithJSON вспомогательная функция для отправки JSON-ответов.
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, message string, handlerName string) {
	metrics.HttpRequestsTotal.WithLabelValues(handlerName, strconv.Itoa(code)).Inc()
	http.Error(w, message, code)
}
