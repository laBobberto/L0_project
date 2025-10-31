package config

import (
	"log"
	"sync"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

// KafkaConfig содержит настройки для подключения к Kafka.
type KafkaConfig struct {
	Brokers  []string `env:"KAFKA_BROKERS" env-default:"localhost:9092"`
	Topic    string   `env:"KAFKA_TOPIC" env-default:"orders"`
	DLQTopic string   `env:"KAFKA_DLQ_TOPIC" env-default:"orders_dlq"` // Топик для "битых" сообщений
	GroupID  string   `env:"KAFKA_GROUP_ID" env-default:"orders-group"`
}

// Config содержит всю конфигурацию приложения.
type Config struct {
	HTTP struct {
		Port string `env:"HTTP_PORT" env-default:"8081"`
	}
	Postgres struct {
		URL string `env:"POSTGRES_URL" env-default:"postgres://user:password@localhost:5432/orders_db?sslmode=disable"`
	}
	Kafka KafkaConfig
	Cache struct {
		Size int `env:"CACHE_SIZE" env-default:"100"`
	}
}

var (
	cfg  Config
	once sync.Once
)

// Get возвращает синглтон-экземпляр конфигурации.
func Get() *Config {
	once.Do(func() {
		if err := godotenv.Load(); err != nil {
			log.Println("Предупреждение: не удалось загрузить файл .env. Используются только переменные окружения.")
		}
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			log.Fatalf("Не удалось прочитать переменные окружения: %v", err)
		}
	})
	return &cfg
}
