package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/GGmuzem/yandex-project/pkg/models"
)

// Путь к файлу базы данных по умолчанию
const defaultDbFilePath = "./calculator.db"

// getDbFilePath возвращает путь к файлу базы данных, учитывая переменную окружения
func getDbFilePath() string {
	// Проверяем наличие переменной окружения DB_PATH
	dbPath := os.Getenv("DB_PATH")
	if dbPath != "" {
		log.Printf("Using database path from environment: %s", dbPath)
		return dbPath
	}
	return defaultDbFilePath
}

// DB представляет соединение с базой данных
type DB struct {
	*sql.DB
}

// New создает новое соединение с базой данных
func New() (*DB, error) {
	// Получаем путь к файлу базы данных
	dbFilePath := getDbFilePath()
	
	// Убедимся, что директория существует
	dir := filepath.Dir(dbFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("error creating directory for database: %w", err)
	}

	// Открываем соединение с базой данных
	db, err := sql.Open("sqlite3", dbFilePath)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	// Возвращаем обертку над соединением
	return &DB{DB: db}, nil
}

// CreateTables создает необходимые таблицы в базе данных, если они не существуют
func (db *DB) CreateTables() error {
	// Таблица пользователей
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)
	`)
	if err != nil {
		return fmt.Errorf("error creating users table: %w", err)
	}

	// Таблица выражений
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS expressions (
		id TEXT PRIMARY KEY,
		user_id INTEGER,
		expression TEXT NOT NULL,
		status TEXT NOT NULL,
		result REAL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		completed_at TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	)
	`)
	if err != nil {
		return fmt.Errorf("error creating expressions table: %w", err)
	}

	// Таблица задач
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY,
		expression_id TEXT NOT NULL,
		arg1 TEXT NOT NULL,
		arg2 TEXT NOT NULL,
		operation TEXT NOT NULL,
		status TEXT NOT NULL,
		result REAL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		completed_at TIMESTAMP,
		FOREIGN KEY (expression_id) REFERENCES expressions(id)
	)
	`)
	if err != nil {
		return fmt.Errorf("error creating tasks table: %w", err)
	}

	log.Println("Database tables created successfully")
	return nil
}

// SaveExpression сохраняет новое выражение в базу данных
func (db *DB) SaveExpression(expr models.Expression) error {
	// Добавляем user_id в запрос, чтобы выражения привязывались к пользователю
	_, err := db.Exec(`
	INSERT INTO expressions (id, expression, status, user_id, created_at) 
	VALUES (?, ?, ?, ?, ?)
	`, expr.ID, expr.Expression, expr.Status, expr.UserID, expr.CreatedAt)
	if err != nil {
		return fmt.Errorf("error saving expression: %w", err)
	}
	return nil
}

// UpdateExpressionStatus обновляет статус выражения
func (db *DB) UpdateExpressionStatus(id string, status string, result float64) error {
	var err error
	if status == "completed" {
		_, err = db.Exec(`
		UPDATE expressions 
		SET status = ?, result = ?, completed_at = CURRENT_TIMESTAMP 
		WHERE id = ?
		`, status, result, id)
	} else {
		_, err = db.Exec(`
		UPDATE expressions 
		SET status = ? 
		WHERE id = ?
		`, status, id)
	}

	if err != nil {
		return fmt.Errorf("error updating expression status: %w", err)
	}
	return nil
}

// SaveTask сохраняет задачу в базу данных
func (db *DB) SaveTask(task models.Task) error {
	_, err := db.Exec(`
	INSERT INTO tasks (id, expression_id, arg1, arg2, operation, status) 
	VALUES (?, ?, ?, ?, ?, ?)
	`, task.ID, task.ExpressionID, task.Arg1, task.Arg2, task.Operation, "pending")
	if err != nil {
		return fmt.Errorf("error saving task: %w", err)
	}
	return nil
}

// UpdateTaskResult обновляет результат задачи
func (db *DB) UpdateTaskResult(id int, result float64) error {
	_, err := db.Exec(`
	UPDATE tasks 
	SET status = 'completed', result = ?, completed_at = CURRENT_TIMESTAMP 
	WHERE id = ?
	`, result, id)
	if err != nil {
		return fmt.Errorf("error updating task result: %w", err)
	}
	return nil
}

// GetTaskResult возвращает результат выполнения задачи по её ID
func (db *DB) GetTaskResult(id int) (float64, bool, error) {
	var result float64
	var status string
	err := db.QueryRow(`
	SELECT result, status FROM tasks WHERE id = ?
	`, id).Scan(&result, &status)
	
	if err == sql.ErrNoRows {
		return 0, false, nil
	} else if err != nil {
		return 0, false, fmt.Errorf("error getting task result: %w", err)
	}
	
	return result, status == "completed", nil
}

// GetAllTasksForExpression возвращает все задачи для указанного выражения
func (db *DB) GetAllTasksForExpression(expressionID string) ([]models.Task, error) {
	rows, err := db.Query(`
	SELECT id, expression_id, arg1, arg2, operation, status, result 
	FROM tasks 
	WHERE expression_id = ?
	`, expressionID)
	if err != nil {
		return nil, fmt.Errorf("error getting tasks for expression: %w", err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var task models.Task
		var status string
		var result sql.NullFloat64
		err := rows.Scan(&task.ID, &task.ExpressionID, &task.Arg1, &task.Arg2, &task.Operation, &status, &result)
		if err != nil {
			return nil, fmt.Errorf("error scanning task row: %w", err)
		}

		task.IsFinished = status == "completed"
		if result.Valid {
			task.Result = result.Float64
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating task rows: %w", err)
	}

	return tasks, nil
}

// GetExpression возвращает выражение по ID
func (db *DB) GetExpression(expressionID string) (models.Expression, error) {
	var expr models.Expression
	var resultNull sql.NullFloat64

	err := db.QueryRow(`
	SELECT id, expression, status, result 
	FROM expressions 
	WHERE id = ?
	`, expressionID).Scan(&expr.ID, &expr.Expression, &expr.Status, &resultNull)
	
	if err != nil {
		return expr, fmt.Errorf("error getting expression: %w", err)
	}

	if resultNull.Valid {
		expr.Result = resultNull.Float64
	}

	return expr, nil
}


// GetExpressionsByUser возвращает все выражения пользователя
func (db *DB) GetExpressionsByUser(userID int, limit, offset int) ([]models.Expression, int, error) {
	var expressions []models.Expression
	
	// Получаем общее количество выражений пользователя
	countQuery := `SELECT COUNT(*) FROM expressions WHERE user_id = ?`
	var total int
	err := db.QueryRow(countQuery, userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка при подсчете выражений: %w", err)
	}
	
	query := `
	SELECT id, expression, status, result, user_id, created_at 
	FROM expressions 
	WHERE user_id = ? 
	ORDER BY created_at DESC 
	LIMIT ? OFFSET ?`

	rows, err := db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка при получении выражений: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var expr models.Expression
		var userIDDb sql.NullInt64
		var createdAt sql.NullString
		
		err := rows.Scan(&expr.ID, &expr.Expression, &expr.Status, &expr.Result, &userIDDb, &createdAt)
		if err != nil {
			return nil, 0, fmt.Errorf("ошибка при сканировании выражения: %w", err)
		}
		
		expressions = append(expressions, expr)
	}
	
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("ошибка при итерации результатов: %w", err)
	}
	
	return expressions, total, nil
}

// SaveUser сохраняет нового пользователя в базу данных
func (db *DB) SaveUser(username, passwordHash string) (int, error) {
	// Проверяем, что пользователь с таким именем не существует
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("ошибка при проверке существования пользователя: %w", err)
	}
	
	if count > 0 {
		return 0, fmt.Errorf("пользователь с именем %s уже существует", username)
	}
	
	// Сохраняем пользователя
	result, err := db.Exec(
		"INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, datetime('now'))",
		username, passwordHash,
	)
	if err != nil {
		return 0, fmt.Errorf("ошибка при сохранении пользователя: %w", err)
	}
	
	userID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ошибка при получении ID пользователя: %w", err)
	}
	
	return int(userID), nil
}

// GetUserByUsername получает пользователя по имени
func (db *DB) GetUserByUsername(username string) (models.User, error) {
	var user models.User
	
	query := "SELECT id, username, password_hash FROM users WHERE username = ?"
	err := db.QueryRow(query, username).Scan(&user.ID, &user.Username, &user.PasswordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return user, fmt.Errorf("пользователь с именем %s не найден", username)
		}
		return user, fmt.Errorf("ошибка при получении пользователя: %w", err)
	}
	
	return user, nil
}

// GetUserByID получает пользователя по ID
func (db *DB) GetUserByID(id int) (models.User, error) {
	var user models.User
	
	query := "SELECT id, username, password_hash FROM users WHERE id = ?"
	err := db.QueryRow(query, id).Scan(&user.ID, &user.Username, &user.PasswordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return user, fmt.Errorf("пользователь с ID %d не найден", id)
		}
		return user, fmt.Errorf("ошибка при получении пользователя: %w", err)
	}
	
	return user, nil
}
