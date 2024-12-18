package main

import (
	"log"
	"net/http"

	"calc_service/internal/handlers"
)

func main() {
	// Регистрируем обработчики
	http.HandleFunc("/api/v1/calculate", handlers.CalculateHandler)

	// Запускаем сервер
	log.Println("Сервис запущен на порту 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}
}
