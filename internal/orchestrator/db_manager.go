package orchestrator

import (
	"log"
	"strconv"

	"github.com/GGmuzem/yandex-project/internal/database"
	"github.com/GGmuzem/yandex-project/pkg/models"
)

// DBManager управляет взаимодействием оркестратора с базой данных
type DBManager struct {
	db *database.DB
}

// NewDBManager создает новый экземпляр менеджера базы данных
func NewDBManager() (*DBManager, error) {
	db, err := database.New()
	if err != nil {
		return nil, err
	}

	if err := db.CreateTables(); err != nil {
		return nil, err
	}

	return &DBManager{db: db}, nil
}

// SaveExpression сохраняет новое выражение в БД
func (m *DBManager) SaveExpression(expr models.Expression) error {
	return m.db.SaveExpression(expr)
}

// UpdateExpressionStatus обновляет статус выражения
func (m *DBManager) UpdateExpressionStatus(id string, status string, result float64) error {
	return m.db.UpdateExpressionStatus(id, status, result)
}

// SaveTask сохраняет задачу в БД
func (m *DBManager) SaveTask(task models.Task) error {
	return m.db.SaveTask(task)
}

// UpdateTaskResult обновляет результат задачи
func (m *DBManager) UpdateTaskResult(id int, result float64) error {
	return m.db.UpdateTaskResult(id, result)
}

// GetTaskResult возвращает результат задачи по ID
func (m *DBManager) GetTaskResult(id int) (float64, bool, error) {
	return m.db.GetTaskResult(id)
}

// GetDependentTaskResult получает результат задачи, от которой зависит текущая задача
func (m *DBManager) GetDependentTaskResult(arg string) (float64, bool) {
	// Проверяем, ссылается ли аргумент на результат другой задачи
	if len(arg) > 6 && arg[:6] == "result" {
		// Извлекаем ID задачи из строки вида "resultXXX"
		taskID, err := strconv.Atoi(arg[6:])
		if err != nil {
			log.Printf("Ошибка при разборе ID задачи из '%s': %v", arg, err)
			return 0, false
		}

		// Получаем результат из БД
		result, completed, err := m.db.GetTaskResult(taskID)
		if err != nil {
			log.Printf("Ошибка при получении результата задачи #%d: %v", taskID, err)
			return 0, false
		}

		return result, completed
	}

	return 0, false
}

// GetTasksForExpression возвращает все задачи для указанного выражения
func (m *DBManager) GetTasksForExpression(expressionID string) ([]models.Task, error) {
	return m.db.GetAllTasksForExpression(expressionID)
}

// GetExpression возвращает выражение по ID
func (m *DBManager) GetExpression(expressionID string) (models.Expression, error) {
	return m.db.GetExpression(expressionID)
}

// Close закрывает соединение с базой данных
func (m *DBManager) Close() error {
	return m.db.Close()
}

// SaveUser сохраняет нового пользователя в базу данных
func (m *DBManager) SaveUser(username, passwordHash string) (int, error) {
	return m.db.SaveUser(username, passwordHash)
}

// GetUserByUsername получает пользователя по имени
func (m *DBManager) GetUserByUsername(username string) (models.User, error) {
	return m.db.GetUserByUsername(username)
}

// GetUserByID получает пользователя по ID
func (m *DBManager) GetUserByID(id int) (models.User, error) {
	return m.db.GetUserByID(id)
}

// GetExpressionsByUser возвращает выражения пользователя
func (m *DBManager) GetExpressionsByUser(userID int, limit, offset int) ([]models.Expression, int, error) {
	return m.db.GetExpressionsByUser(userID, limit, offset)
}
