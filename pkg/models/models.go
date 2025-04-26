package models

type Expression struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	Result    float64 `json:"result,omitempty"`
	UserID    int     `json:"user_id,omitempty"`
	CreatedAt int64   `json:"created_at,omitempty"`
}

type Task struct {
	ID            int    `json:"id"`
	Arg1          string `json:"arg1"`
	Arg2          string `json:"arg2"`
	Operation     string `json:"operation"`
	OperationTime int    `json:"operation_time"`
	ExpressionID  string `json:"expression_id,omitempty"`
}

type TaskResult struct {
	ID     int     `json:"id"`
	Result float64 `json:"result"`
}

// User представляет пользователя системы
type User struct {
	ID       int    `json:"id"`
	Login    string `json:"login"`
	Password string `json:"-"` // Не сериализуем пароль в JSON
}

// LoginRequest используется для запроса на вход
type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// LoginResponse ответ на успешный вход
type LoginResponse struct {
	Token string `json:"token"`
}

// RegisterRequest используется для регистрации
type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}
