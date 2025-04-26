package database

import (
	"github.com/GGmuzem/yandex-project/pkg/models"
)

// Database интерфейс для работы с хранилищем данных
type Database interface {
	// Миграция и управление соединением
	MigrateDB() error
	Close() error

	// Методы для работы с пользователями
	UserExists(login string) (bool, error)
	CreateUser(user *models.User) (int, error)
	GetUserByLogin(login string) (*models.User, error)

	// Методы для работы с выражениями
	SaveExpression(expr *models.Expression) error
	UpdateExpressionStatus(id string, status string, result float64) error
	GetExpression(id string, userID int) (*models.Expression, error)
	GetExpressions(userID int) ([]*models.Expression, error)

	// Методы для работы с результатами вычислений
	SaveResult(taskID int, result float64, exprID string) error
	GetResult(taskID int) (float64, error)
	GetResultsByExprID(exprID string) (map[int]float64, error)
}
