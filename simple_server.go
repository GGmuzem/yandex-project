package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// API маршруты
	// Регистрация
	http.HandleFunc("/api/v1/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		var data struct {
			Login    string `json:"login"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "Ошибка разбора JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("Регистрация пользователя: %s", data.Login)

		// Возвращаем ID пользователя
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":1,"login":"%s"}`, data.Login)
	})

	// Авторизация
	http.HandleFunc("/api/v1/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		var data struct {
			Login    string `json:"login"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "Ошибка разбора JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("Авторизация пользователя: %s", data.Login)

		// Возвращаем токен
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"token":"test-token-123"}`)
	})

	// Расчет выражения
	http.HandleFunc("/api/v1/calculate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		// Проверяем токен
		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, "Отсутствует токен авторизации", http.StatusUnauthorized)
			return
		}

		var data struct {
			Expression string `json:"expression"`
		}

		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "Ошибка разбора JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("Расчет выражения: %s", data.Expression)

		// Возвращаем ID выражения
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"expr-123"}`)
	})

	// Получение выражения
	http.HandleFunc("/api/v1/expressions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		// Проверяем токен
		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, "Отсутствует токен авторизации", http.StatusUnauthorized)
			return
		}

		// ID выражения
		exprID := r.URL.Path[len("/api/v1/expressions/"):]

		log.Printf("Получение выражения: %s", exprID)

		// Возвращаем выражение
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"%s","status":"completed","result":4}`, exprID)
	})

	// Получение списка выражений
	http.HandleFunc("/api/v1/expressions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		// Проверяем токен
		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, "Отсутствует токен авторизации", http.StatusUnauthorized)
			return
		}

		log.Printf("Получение списка выражений")

		// Возвращаем список выражений
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"id":"expr-123","status":"completed","result":4}]`)
	})

	// Проверка статуса сервера
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// Запускаем сервер
	serverAddr := ":8080"
	log.Printf("Простой сервер запущен на http://localhost%s", serverAddr)

	// Запускаем HTTP сервер в отдельной горутине
	go func() {
		if err := http.ListenAndServe(serverAddr, nil); err != nil {
			log.Fatalf("Ошибка запуска HTTP сервера: %v", err)
		}
	}()

	// Ожидаем сигнала для завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("Сервер остановлен")
}
