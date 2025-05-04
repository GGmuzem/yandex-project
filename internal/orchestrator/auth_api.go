package orchestrator

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/GGmuzem/yandex-project/internal/auth"
	"github.com/GGmuzem/yandex-project/internal/database"
	"github.com/GGmuzem/yandex-project/pkg/models"
)

// AuthHandlers содержит обработчики для аутентификации
type AuthHandlers struct {
	DB database.Database
}

// NewAuthHandlers создает новый экземпляр обработчиков аутентификации
func NewAuthHandlers(db database.Database) *AuthHandlers {
	return &AuthHandlers{
		DB: db,
	}
}

// RegisterHandler обработчик регистрации нового пользователя
func (h *AuthHandlers) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("RegisterHandler: Неверный метод: %s", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	log.Printf("RegisterHandler: Получен запрос на регистрацию")
	log.Printf("RegisterHandler: Content-Type: %s", r.Header.Get("Content-Type"))
	log.Printf("RegisterHandler: Content-Length: %s", r.Header.Get("Content-Length"))

	// Чтение тела запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("RegisterHandler: Ошибка чтения тела запроса: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request data"})
		return
	}
	defer r.Body.Close()

	log.Printf("RegisterHandler: Тело запроса: %s", string(body))

	// Если тело пустое, возвращаем ошибку
	if len(body) == 0 {
		log.Printf("RegisterHandler: Пустое тело запроса")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request data"})
		return
	}

	// Декодирование JSON
	var req models.RegisterRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		log.Printf("RegisterHandler: Ошибка декодирования JSON: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request data"})
		return
	}

	log.Printf("RegisterHandler: Получены данные пользователя для регистрации login=%s", req.Login)

	// Проверка наличия логина и пароля
	if req.Login == "" || req.Password == "" {
		log.Printf("RegisterHandler: Логин или пароль отсутствуют")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Login and password are required"})
		return
	}

	// Создание нового пользователя
	err = auth.RegisterUser(h.DB, &req)
	if err != nil {
		log.Printf("RegisterHandler: Ошибка при регистрации пользователя: %v", err)
		w.Header().Set("Content-Type", "application/json")
		if err == auth.ErrUserExists {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "User already exists"})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		}
		return
	}

	// Успешная регистрация
	log.Printf("RegisterHandler: Пользователь %s успешно зарегистрирован", req.Login)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "User registered successfully"})
}

// LoginHandler обработчик входа пользователя
func (h *AuthHandlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request data"})
		return
	}

	// Валидация данных
	if req.Login == "" || req.Password == "" {
		http.Error(w, "Login and password are required", http.StatusUnprocessableEntity)
		return
	}

	// Аутентифицируем пользователя
	token, err := auth.LoginUser(h.DB, &req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err == auth.ErrInvalidCredentials {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid login or password"})
		} else {
			log.Printf("Error authenticating user: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		}
		return
	}

	// Отправляем токен
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.LoginResponse{Token: token})
}

// AuthMiddleware промежуточное ПО для проверки аутентификации
func (h *AuthHandlers) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return auth.AuthMiddleware(h.DB, next)
}

// CalculateWithAuthHandler обработчик вычисления выражения с аутентификацией
func (h *AuthHandlers) CalculateWithAuthHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем пользователя из контекста (установлен middleware)
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	var input struct {
		Expression string `json:"expression"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Expression == "" {
		http.Error(w, "Invalid data", http.StatusUnprocessableEntity)
		return
	}

	// Используем функцию из Manager для генерации ID
	exprID := GenerateUniqueExpressionID()

	// Создаем выражение с ID пользователя
	expr := &models.Expression{
		ID:        exprID,
		Status:    "pending",
		UserID:    user.ID,
		CreatedAt: time.Now().Unix(),
	}

	// Сохраняем выражение в БД
	if err := h.DB.SaveExpression(expr); err != nil {
		log.Printf("Error saving expression: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}

	// Используем глобальный менеджер задач
	Manager.mu.Lock()
	Manager.Expressions[exprID] = expr
	Manager.mu.Unlock()

	go func() {
		log.Printf("Парсинг выражения: %s для пользователя %s", input.Expression, user.Login)
		taskList := ParseExpression(input.Expression)

		log.Printf("Создание задач для выражения %s. Всего задач: %d", exprID, len(taskList))

		// Отображаем все созданные задачи
		log.Printf("Структура созданных задач для выражения %s:", exprID)
		for i, t := range taskList {
			log.Printf("Задача %d: ID=%d, %s %s %s, Готова: %v",
				i+1, t.ID, t.Arg1, t.Operation, t.Arg2, isTaskReady(t))
		}

		// Добавляем задачи в менеджер
		Manager.AddExpression(exprID, taskList)

		// Обновляем статус выражения
		UpdateExpressions()
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": exprID})
}

// ListExpressionsWithAuthHandler обработчик списка выражений с аутентификацией
func (h *AuthHandlers) ListExpressionsWithAuthHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем пользователя из контекста
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// Получаем выражения пользователя из БД
	exprList, err := h.DB.GetExpressions(user.ID)
	if err != nil {
		log.Printf("Error getting expressions: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}

	// Преобразуем в формат для ответа
	expressions := make([]models.Expression, 0, len(exprList))
	for _, expr := range exprList {
		expressions = append(expressions, *expr)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string][]models.Expression{"expressions": expressions})
}

// GetExpressionWithAuthHandler обработчик получения выражения с аутентификацией
func (h *AuthHandlers) GetExpressionWithAuthHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем пользователя из контекста
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// Получаем ID выражения из URL
	id := getExpressionIDFromURL(r.URL.Path)
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid expression ID"})
		return
	}

	// Получаем выражение из БД
	expr, err := h.DB.GetExpression(id, user.ID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Expression not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]models.Expression{"expression": *expr})
}
