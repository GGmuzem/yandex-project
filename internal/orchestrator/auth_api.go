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
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("RegisterHandler: Получен запрос на регистрацию")
	log.Printf("RegisterHandler: Content-Type: %s", r.Header.Get("Content-Type"))
	log.Printf("RegisterHandler: Content-Length: %s", r.Header.Get("Content-Length"))

	// Чтение тела запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("RegisterHandler: Ошибка чтения тела запроса: %v", err)
		http.Error(w, "Invalid request data", http.StatusUnprocessableEntity)
		return
	}
	defer r.Body.Close()

	log.Printf("RegisterHandler: Тело запроса: %s", string(body))

	// Если тело пустое, возвращаем ошибку
	if len(body) == 0 {
		log.Printf("RegisterHandler: Пустое тело запроса")
		http.Error(w, "Invalid request data", http.StatusUnprocessableEntity)
		return
	}

	// Декодирование JSON
	var req models.RegisterRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		log.Printf("RegisterHandler: Ошибка декодирования JSON: %v", err)
		http.Error(w, "Invalid request data", http.StatusUnprocessableEntity)
		return
	}

	log.Printf("RegisterHandler: Получены данные пользователя для регистрации login=%s", req.Login)

	// Проверка наличия логина и пароля
	if req.Login == "" || req.Password == "" {
		log.Printf("RegisterHandler: Логин или пароль отсутствуют")
		http.Error(w, "Login and password are required", http.StatusBadRequest)
		return
	}

	// Создание нового пользователя
	err = auth.RegisterUser(h.DB, &req)
	if err != nil {
		log.Printf("RegisterHandler: Ошибка при регистрации пользователя: %v", err)
		if err == auth.ErrUserExists {
			http.Error(w, "User already exists", http.StatusConflict)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Успешная регистрация
	log.Printf("RegisterHandler: Пользователь %s успешно зарегистрирован", req.Login)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("User registered successfully"))
}

// LoginHandler обработчик входа пользователя
func (h *AuthHandlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request data", http.StatusUnprocessableEntity)
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
		if err == auth.ErrInvalidCredentials {
			http.Error(w, "Invalid login or password", http.StatusUnauthorized)
		} else {
			log.Printf("Error authenticating user: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		http.Error(w, "Internal server error", http.StatusInternalServerError)
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем выражения пользователя из БД
	exprList, err := h.DB.GetExpressions(user.ID)
	if err != nil {
		log.Printf("Error getting expressions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем ID выражения из URL
	id := getExpressionIDFromURL(r.URL.Path)
	if id == "" {
		http.Error(w, "Invalid expression ID", http.StatusBadRequest)
		return
	}

	// Получаем выражение из БД
	expr, err := h.DB.GetExpression(id, user.ID)
	if err != nil {
		http.Error(w, "Expression not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]models.Expression{"expression": *expr})
}
