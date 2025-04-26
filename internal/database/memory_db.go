package database

import (
	"fmt"
	"sync"
	"time"

	"github.com/GGmuzem/yandex-project/pkg/models"
	"golang.org/x/crypto/bcrypt"
)

// MemoryDB реализация БД в памяти без использования SQLite
type MemoryDB struct {
	users       map[string]*models.User
	expressions map[string]*models.Expression
	results     map[int]float64
	userByID    map[int]*models.User
	mutex       sync.RWMutex
	userIDSeq   int
}

// NewMemoryDB создает новую in-memory БД
func NewMemoryDB() *MemoryDB {
	return &MemoryDB{
		users:       make(map[string]*models.User),
		expressions: make(map[string]*models.Expression),
		results:     make(map[int]float64),
		userByID:    make(map[int]*models.User),
		userIDSeq:   1,
	}
}

// Close просто заглушка для совместимости
func (db *MemoryDB) Close() error {
	return nil
}

// MigrateDB для in-memory не требуется миграция
func (db *MemoryDB) MigrateDB() error {
	return nil
}

// UserExists проверяет существование пользователя с указанным логином
func (db *MemoryDB) UserExists(login string) (bool, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	_, exists := db.users[login]
	return exists, nil
}

// CreateUser создает нового пользователя
func (db *MemoryDB) CreateUser(user *models.User) (int, error) {
	// Проверяем, существует ли пользователь
	exists, _ := db.UserExists(user.Login)
	if exists {
		return 0, fmt.Errorf("пользователь с логином %s уже существует", user.Login)
	}

	// Хешируем пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	// Назначаем ID
	userID := db.userIDSeq
	db.userIDSeq++

	// Создаем копию пользователя с хешированным паролем
	newUser := &models.User{
		ID:       userID,
		Login:    user.Login,
		Password: string(hashedPassword),
	}

	// Сохраняем пользователя
	db.users[user.Login] = newUser
	db.userByID[userID] = newUser

	return userID, nil
}

// GetUserByLogin возвращает пользователя по логину
func (db *MemoryDB) GetUserByLogin(login string) (*models.User, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	user, exists := db.users[login]
	if !exists {
		return nil, fmt.Errorf("пользователь с логином %s не найден", login)
	}
	return user, nil
}

// SaveExpression сохраняет выражение в БД
func (db *MemoryDB) SaveExpression(expr *models.Expression) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	// Создаем копию выражения
	newExpr := &models.Expression{
		ID:        expr.ID,
		Status:    expr.Status,
		UserID:    expr.UserID,
		CreatedAt: time.Now().Unix(),
	}

	db.expressions[expr.ID] = newExpr
	return nil
}

// UpdateExpressionStatus обновляет статус выражения
func (db *MemoryDB) UpdateExpressionStatus(id string, status string, result float64) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	expr, exists := db.expressions[id]
	if !exists {
		return fmt.Errorf("выражение с ID %s не найдено", id)
	}

	expr.Status = status
	expr.Result = result
	return nil
}

// GetExpression возвращает выражение по ID и user_id
func (db *MemoryDB) GetExpression(id string, userID int) (*models.Expression, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	expr, exists := db.expressions[id]
	if !exists || expr.UserID != userID {
		return nil, fmt.Errorf("выражение с ID %s не найдено", id)
	}
	return expr, nil
}

// GetExpressions возвращает все выражения пользователя
func (db *MemoryDB) GetExpressions(userID int) ([]*models.Expression, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	expressions := []*models.Expression{}
	for _, expr := range db.expressions {
		if expr.UserID == userID {
			expressions = append(expressions, expr)
		}
	}
	return expressions, nil
}

// SaveResult сохраняет результат вычисления задачи
func (db *MemoryDB) SaveResult(taskID int, result float64, exprID string) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	db.results[taskID] = result
	return nil
}

// GetResult возвращает результат задачи по ID
func (db *MemoryDB) GetResult(taskID int) (float64, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	result, exists := db.results[taskID]
	if !exists {
		return 0, fmt.Errorf("результат для задачи %d не найден", taskID)
	}
	return result, nil
}

// GetResultsByExprID возвращает все результаты для выражения
func (db *MemoryDB) GetResultsByExprID(exprID string) (map[int]float64, error) {
	// Для упрощенной in-memory версии возвращаем пустую карту
	return make(map[int]float64), nil
}
