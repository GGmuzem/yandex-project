package orchestrator

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/GGmuzem/yandex-project/pkg/models"
	"github.com/gorilla/mux"
)

// Ответ для списка выражений с пагинацией
type ExpressionsResponse struct {
	Expressions []models.Expression `json:"expressions"`
	Total       int                 `json:"total"`
	Page        int                 `json:"page"`
	PageSize    int                 `json:"page_size"`
}

// GetUserExpressionsHandler обрабатывает запросы на получение списка выражений пользователя
func GetUserExpressionsHandler(w http.ResponseWriter, r *http.Request) {
	// Устанавливаем CORS заголовки для корректной работы в браузере
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	
	// Обрабатываем OPTIONS запросы для CORS
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Получаем ID пользователя из контекста (устанавливается в JWT middleware)
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"error": "Необходима авторизация"}`)
		return
	}

	// Получаем параметры пагинации
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")

	page := 1
	pageSize := 10

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// Вычисляем offset для SQL запроса
	offset := (page - 1) * pageSize

	// Получаем выражения пользователя из базы данных
	expressions, total, err := dbManager.GetExpressionsByUser(userID, pageSize, offset)
	if err != nil {
		log.Printf("Ошибка при получении выражений пользователя: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error": "Ошибка при получении истории выражений"}`)
		return
	}

	// Формируем ответ
	response := ExpressionsResponse{
		Expressions: expressions,
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
	}

	// Сериализуем и отправляем ответ
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Ошибка при сериализации ответа: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error": "Ошибка сервера"}`)
		return
	}
}

// GetExpressionTasksHandler возвращает список задач для конкретного выражения
func GetExpressionTasksHandler(w http.ResponseWriter, r *http.Request) {
	// Устанавливаем CORS заголовки для корректной работы в браузере
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	
	// Обрабатываем OPTIONS запросы для CORS
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Получаем ID пользователя из контекста (устанавливается в JWT middleware)
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"error": "Необходима авторизация"}`)
		return
	}

	// Получаем ID выражения из URL
	vars := mux.Vars(r)
	expressionID := vars["id"]
	if expressionID == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error": "Не указан ID выражения"}`)
		return
	}

	// Получаем выражение из БД для проверки прав доступа
	expression, err := dbManager.GetExpression(expressionID)
	if err != nil {
		log.Printf("Ошибка при получении выражения %s: %v", expressionID, err)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error": "Выражение не найдено"}`)
		return
	}

	// Проверяем, что выражение принадлежит пользователю
	if expression.UserID != userID {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"error": "Доступ запрещен"}`)
		return
	}

	// Получаем задачи для выражения
	tasks, err := dbManager.GetTasksForExpression(expressionID)
	if err != nil {
		log.Printf("Ошибка при получении задач для выражения %s: %v", expressionID, err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error": "Не удалось получить задачи выражения"}`)
		return
	}

	// Формируем и отправляем ответ
	response := struct {
		Tasks []models.Task `json:"tasks"`
	}{
		Tasks: tasks,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Ошибка при сериализации задач: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error": "Ошибка сервера"}`)
	}
}
