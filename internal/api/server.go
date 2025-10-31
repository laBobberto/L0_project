package api

import (
	"L0_project/internal/cache"
	"L0_project/internal/database"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Server представляет HTTP-сервер.
type Server struct {
	port    string
	router  *chi.Mux
	storage database.Storage
	cache   cache.Cache
}

// NewServer создает и настраивает новый экземпляр сервера.
func NewServer(port string, storage database.Storage, cache cache.Cache) *Server {
	server := &Server{
		port:    port,
		storage: storage,
		cache:   cache,
	}
	server.router = server.setupRouter()
	return server
}

// Run запускает HTTP-сервер.
func (s *Server) Run() error {
	address := fmt.Sprintf(":%s", s.port)
	fmt.Printf("🚀 HTTP-сервер запущен на http://localhost%s\n", address)
	return http.ListenAndServe(address, s.router)
}

// setupRouter настраивает маршрутизацию.
func (s *Server) setupRouter() *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// Middleware для OpenTelemetry
	router.Use(otelhttp.NewMiddleware("l0-http-server"))

	// Обработчик API
	orderHandler := NewOrderHandler(s.storage, s.cache)
	router.Get("/api/order/{orderUID}", orderHandler.GetByUID)

	// Эндпоинт для сбора метрик Prometheus
	router.Handle("/metrics", promhttp.Handler())

	// Обработчик для статических файлов
	fileServer := http.FileServer(http.Dir("./web/"))
	router.Handle("/*", fileServer)

	return router
}
