package models

// User представляет пользователя системы
type User struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"` // Не передается в JSON
}

type Expression struct {
    ID         string  `json:"id"`
    Expression string  `json:"expression"` // Само выражение
    Status     string  `json:"status"`
    Result     float64 `json:"result,omitempty"`
    UserID     int     `json:"user_id,omitempty"`
    CreatedAt  string  `json:"created_at,omitempty"`
}

type Task struct {
    ID           int    `json:"id"`           // Уникальный ID задачи
    Arg1         string `json:"arg1"`         // Первый аргумент
    Arg2         string `json:"arg2"`         // Второй аргумент
    Operation    string `json:"operation"`    // Операция
    OperationTime int    `json:"operation_time"` // Время выполнения операции в миллисекундах
    
    // Дополнительные поля из примера
    ExpressionID string `json:"expression_id"` // ID выражения, к которому относится задача
    IsFinished   bool   `json:"is_finished"`   // Флаг завершения
    IsWrong      bool   `json:"is_wrong"`      // Флаг ошибки
    Comment      string `json:"comment"`      // Комментарий
    Result       float64 `json:"result"`       // Результат вычисления
}

type TaskResult struct {
    ID     int     `json:"id"`     // ID задачи
    Result float64 `json:"result"` // Результат
}

// TaskResponse представляет ответ на запрос агента для получения задачи
type TaskResponse struct {
    Task  Task   `json:"task"`
    Error string `json:"error,omitempty"`
}

// ExpressionResponse представляет ответ на запрос создания выражения
type ExpressionResponse struct {
    ID    string `json:"id"`
    Error string `json:"error,omitempty"`
}