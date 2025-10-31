package metrics

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HttpRequestsTotal - Счетчик HTTP-запросов
	HttpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Количество HTTP запросов",
		},
		[]string{"handler", "status"}, // Метки: хэндлер и http-статус
	)

	// HttpRequestDuration - Гистограмма длительности HTTP-запросов
	HttpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "Длительность HTTP запросов",
		},
		[]string{"handler"}, // Метки: хэндлер
	)

	// CacheHits - Счетчик попаданий в кэш
	CacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Количество попаданий в кэш",
		},
	)

	// CacheMisses - Счетчик промахов кэша
	CacheMisses = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Количество промахов кэша",
		},
	)

	// KafkaMessagesProcessed - Счетчик обработанных Kafka-сообщений
	KafkaMessagesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_messages_processed_total",
			Help: "Количество обработанных сообщений Kafka",
		},
		[]string{"status"}, // Метки: "success", "dlq_validation", "dlq_db_error", "dlq_failed_write"
	)

	// DBErrors - Счетчик ошибок базы данных
	DBErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_errors_total",
			Help: "Количество ошибок при работе с БД",
		},
		[]string{"operation"}, // Метки: "save_order", "get_order", "get_all", "get_items"
	)

	// CacheSize - Датчик (Gauge) текущего размера кэша
	CacheSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "cache_size_items",
			Help: "Текущий размер кэша в элементах",
		},
	)

	// CacheEvictions - Счетчик вытеснений из кэша (LRU)
	CacheEvictions = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_evictions_total",
			Help: "Количество вытесненных из кэша элементов",
		},
	)
)

// Init используется для регистрации метрик.
// promauto регистрирует их автоматически при создании.
func Init() {
	log.Println("Prometheus метрики инициализированы.")
}
