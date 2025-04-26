package main

import (
	"log"
	"os"
	"strconv"

	"github.com/GGmuzem/yandex-project/internal/agent"
)

func main() {
	log.Println("Agent starting...")

	// Получение количества воркеров из переменной окружения
	power, err := strconv.Atoi(os.Getenv("COMPUTING_POWER"))
	if err != nil || power <= 0 {
		power = 3 // По умолчанию 3 воркера
		log.Printf("COMPUTING_POWER не указано или некорректно, используем значение по умолчанию: %d", power)
	}

	// Получение адреса gRPC сервера из переменной окружения
	grpcServer := os.Getenv("GRPC_SERVER")
	if grpcServer == "" {
		grpcPort := os.Getenv("GRPC_PORT")
		if grpcPort == "" {
			grpcPort = "50052" // По умолчанию порт 50052
		}
		grpcServer = "localhost:" + grpcPort
		log.Printf("GRPC_SERVER не указано, используем значение по умолчанию: %s", grpcServer)
	}

	// Запускаем gRPC воркеры
	log.Printf("Запуск %d gRPC воркеров...", power)
	for i := 0; i < power; i++ {
		go agent.StartGRPCWorker(i, grpcServer)
	}

	log.Println("Agent started")

	// Если нужно поддерживать обратную совместимость с HTTP
	if os.Getenv("USE_HTTP") == "true" {
		log.Println("Также запускаем HTTP воркеры для обратной совместимости")
		agent.StartWorker()
	}

	// Бесконечный цикл для поддержания работы агента
	select {}
}
