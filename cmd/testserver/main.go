package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func main() {
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

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&data); err != nil {
			http.Error(w, "Ошибка разбора JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("Регистрация пользователя: %s", data.Login)

		// Проверка наличия login и password
		if data.Login == "" || data.Password == "" {
			http.Error(w, "Login and password are required", http.StatusUnprocessableEntity)
			return
		}

		// Возвращаем ID пользователя
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":1,"login":"%s"}`, data.Login)
	})

	// Проверка статуса сервера
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// Запускаем HTTP сервер
	log.Println("Запуск тестового сервера на порту 8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
