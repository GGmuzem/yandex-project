package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
	"github.com/GGmuzem/yandex-project/pkg/models"
	"golang.org/x/crypto/bcrypt"
)

// SQLiteDB реализация интерфейса Database для SQLite
type SQLiteDB struct {
	db *sql.DB
}

// New создаёт и инициализирует новый экземпляр SQLite БД
func New(dbPath string) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("не удалось подключиться к базе данных: %w", err)
	}

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("не удалось проверить соединение с базой данных: %w", err)
	}

	return &SQLiteDB{db: db}, nil
}

// Close закрывает соединение с БД
func (db *SQLiteDB) Close() error {
	if db.db != nil {
		return db.db.Close()
	}
	return nil
}

// MigrateDB выполняет миграцию базы данных
func (db *SQLiteDB) MigrateDB() error {
	// Удаляем старую таблицу results
	_, err := db.db.Exec(`DROP TABLE IF EXISTS results`)
	if err != nil {
		return fmt.Errorf("не удалось удалить таблицу results: %w", err)
	}

	// Создаем таблицу пользователей
	_, err = db.db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		login TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		created_at INTEGER NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("не удалось создать таблицу users: %w", err)
	}

	// Создаем таблицу выражений
	_, err = db.db.Exec(`
	CREATE TABLE IF NOT EXISTS expressions (
		id TEXT PRIMARY KEY,
		status TEXT NOT NULL,
		result REAL,
		user_id INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users (id)
	)`)
	if err != nil {
		return fmt.Errorf("не удалось создать таблицу expressions: %w", err)
	}

	// Создаем таблицу для хранения результатов вычислений
	_, err = db.db.Exec(`
	CREATE TABLE IF NOT EXISTS results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER NOT NULL UNIQUE,
		result REAL NOT NULL,
		expression_id TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		FOREIGN KEY (expression_id) REFERENCES expressions (id)
	)`)
	if err != nil {
		return fmt.Errorf("не удалось создать таблицу results: %w", err)
	}
	
	// Создаем индекс для ускорения поиска по expression_id
	_, err = db.db.Exec(`CREATE INDEX IF NOT EXISTS idx_results_expression_id ON results(expression_id)`)
	if err != nil {
		return fmt.Errorf("не удалось создать индекс для таблицы results: %w", err)
	}

	return nil
}

// UserExists проверяет существование пользователя с указанным логином
func (db *SQLiteDB) UserExists(login string) (bool, error) {
	var count int
	err := db.db.QueryRow("SELECT COUNT(*) FROM users WHERE login = ?", login).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateUser создает нового пользователя
func (db *SQLiteDB) CreateUser(user *models.User) (int, error) {
	// Проверяем, существует ли пользователь
	exists, err := db.UserExists(user.Login)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, fmt.Errorf("пользователь с логином %s уже существует", user.Login)
	}

	// Хешируем пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	// Сохраняем пользователя
	result, err := db.db.Exec(
		"INSERT INTO users (login, password, created_at) VALUES (?, ?, ?)",
		user.Login, string(hashedPassword), time.Now().Unix(),
	)
	if err != nil {
		return 0, err
	}

	// Получаем ID нового пользователя
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

// GetUserByLogin возвращает пользователя по логину
func (db *SQLiteDB) GetUserByLogin(login string) (*models.User, error) {
	user := &models.User{}
	err := db.db.QueryRow("SELECT id, login, password FROM users WHERE login = ?", login).Scan(
		&user.ID, &user.Login, &user.Password,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("пользователь с логином %s не найден", login)
		}
		return nil, err
	}
	return user, nil
}

// SaveExpression сохраняет выражение в БД
func (db *SQLiteDB) SaveExpression(expr *models.Expression) error {
	_, err := db.db.Exec(
		"INSERT INTO expressions (id, status, user_id, created_at) VALUES (?, ?, ?, ?)",
		expr.ID, expr.Status, expr.UserID, time.Now().Unix(),
	)
	return err
}

// UpdateExpressionStatus обновляет статус выражения
func (db *SQLiteDB) UpdateExpressionStatus(id string, status string, result float64) error {
	log.Printf("=== ОТЛАДКА SQLiteDB.UpdateExpressionStatus: Обновление выражения %s, статус %s, результат %f", id, status, result)
	
	// Проверяем текущее значение в БД перед обновлением
	var currentStatus string
	var currentResult sql.NullFloat64
	err := db.db.QueryRow("SELECT status, result FROM expressions WHERE id = ?", id).Scan(&currentStatus, &currentResult)
	if err != nil {
		log.Printf("=== ОТЛАДКА SQLiteDB.UpdateExpressionStatus: Ошибка при чтении текущего значения: %v", err)
	} else {
		var currentResultValue float64
		if currentResult.Valid {
			currentResultValue = currentResult.Float64
		}
		log.Printf("=== ОТЛАДКА SQLiteDB.UpdateExpressionStatus: Текущее значение в БД: статус %s, результат %f", currentStatus, currentResultValue)
	}
	
	// Проверяем, существует ли выражение
	var exists bool
	err = db.db.QueryRow("SELECT EXISTS(SELECT 1 FROM expressions WHERE id = ?)", id).Scan(&exists)
	if err != nil {
		log.Printf("=== ОТЛАДКА SQLiteDB.UpdateExpressionStatus: Ошибка при проверке существования выражения: %v", err)
		return err
	}
	
	if !exists {
		log.Printf("=== ОТЛАДКА SQLiteDB.UpdateExpressionStatus: Выражение %s не найдено в БД", id)
		return fmt.Errorf("выражение %s не найдено", id)
	}
	
	// Выполняем обновление
	res, err := db.db.Exec(
		"UPDATE expressions SET status = ?, result = ? WHERE id = ?",
		status, result, id,
	)
	
	if err != nil {
		log.Printf("=== ОТЛАДКА SQLiteDB.UpdateExpressionStatus: Ошибка при обновлении: %v", err)
		return err
	}
	
	rowsAffected, _ := res.RowsAffected()
	log.Printf("=== ОТЛАДКА SQLiteDB.UpdateExpressionStatus: Обновлено строк: %d", rowsAffected)
	
	// Проверяем значение после обновления
	err = db.db.QueryRow("SELECT status, result FROM expressions WHERE id = ?", id).Scan(&currentStatus, &currentResult)
	if err != nil {
		log.Printf("=== ОТЛАДКА SQLiteDB.UpdateExpressionStatus: Ошибка при чтении обновленного значения: %v", err)
	} else {
		var currentResultValue float64
		if currentResult.Valid {
			currentResultValue = currentResult.Float64
		}
		log.Printf("=== ОТЛАДКА SQLiteDB.UpdateExpressionStatus: После обновления в БД: статус %s, результат %f", currentStatus, currentResultValue)
	}
	
	return nil
}

// GetExpression возвращает выражение по ID и user_id
func (db *SQLiteDB) GetExpression(id string, userID int) (*models.Expression, error) {
	expr := &models.Expression{}

	var result sql.NullFloat64
	err := db.db.QueryRow(`
		SELECT id, status, result, user_id, created_at 
		FROM expressions 
		WHERE id = ? AND user_id = ?`, id, userID).Scan(
		&expr.ID, &expr.Status, &result, &expr.UserID, &expr.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("выражение с ID %s не найдено", id)
		}
		return nil, err
	}

	if result.Valid {
		expr.Result = result.Float64
	}

	return expr, nil
}

// GetExpressions возвращает все выражения пользователя
func (db *SQLiteDB) GetExpressions(userID int) ([]*models.Expression, error) {
	rows, err := db.db.Query(`
		SELECT id, status, result, user_id, created_at 
		FROM expressions 
		WHERE user_id = ? 
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	expressions := []*models.Expression{}
	for rows.Next() {
		expr := &models.Expression{}
		var result sql.NullFloat64
		if err := rows.Scan(&expr.ID, &expr.Status, &result, &expr.UserID, &expr.CreatedAt); err != nil {
			return nil, err
		}

		if result.Valid {
			expr.Result = result.Float64
		}

		expressions = append(expressions, expr)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return expressions, nil
}

// SaveResult сохраняет результат задачи и обновляет статус выражения, если все задачи завершены
func (db *SQLiteDB) SaveResult(taskID int, result float64, exprID string) error {
	log.Printf("=== ОТЛАДКА SQLiteDB.SaveResult: Задача #%d, результат %f, выражение %s - сохраняем результат", taskID, result, exprID)
	
	// Сохраняем результат задачи в таблицу task_results
	_, err := db.db.Exec(
		"INSERT OR REPLACE INTO task_results (task_id, result, expression_id) VALUES (?, ?, ?)",
		taskID, result, exprID,
	)
	if err != nil {
		log.Printf("=== ОТЛАДКА SQLiteDB.SaveResult: Ошибка при сохранении результата в таблицу task_results: %v", err)
		return err
	}
	log.Printf("=== ОТЛАДКА SQLiteDB.SaveResult: Результат задачи #%d сохранен в таблицу task_results", taskID)
	
	// Удаляем задачу из таблицы tasks, так как она уже выполнена
	_, err = db.db.Exec("DELETE FROM tasks WHERE id = ?", taskID)
	if err != nil {
		log.Printf("=== ОТЛАДКА SQLiteDB.SaveResult: Ошибка при удалении задачи из таблицы tasks: %v", err)
		// Не возвращаем ошибку, так как результат уже сохранен
	} else {
		log.Printf("=== ОТЛАДКА SQLiteDB.SaveResult: Задача #%d удалена из таблицы tasks", taskID)
	}
	
	// Проверяем, остались ли задачи для этого выражения
	var count int
	err = db.db.QueryRow("SELECT COUNT(*) FROM tasks WHERE expression_id = ?", exprID).Scan(&count)
	if err != nil {
		log.Printf("=== ОТЛАДКА SQLiteDB.SaveResult: Ошибка при проверке оставшихся задач: %v", err)
		return err
	}
	
	log.Printf("=== ОТЛАДКА SQLiteDB.SaveResult: Для выражения %s осталось %d невыполненных задач", exprID, count)
	
	// Если задач больше нет, обновляем результат выражения
	if count == 0 {
		// Получаем последний результат для выражения
		var finalResult float64
		row := db.db.QueryRow(
			"SELECT result FROM task_results WHERE expression_id = ? ORDER BY task_id DESC LIMIT 1",
			exprID,
		)
		err = row.Scan(&finalResult)
		if err != nil {
			log.Printf("=== ОТЛАДКА SQLiteDB.SaveResult: Ошибка при получении финального результата: %v", err)
			// Используем текущий результат, если не можем получить последний
			finalResult = result
		}
		
		log.Printf("=== ОТЛАДКА SQLiteDB.SaveResult: Обновляем результат выражения %s на %f", exprID, finalResult)
		
		// Обновляем выражение в БД
		if err := db.UpdateExpressionStatus(exprID, "completed", finalResult); err != nil {
			log.Printf("=== ОТЛАДКА SQLiteDB.SaveResult: Ошибка при обновлении выражения: %v", err)
			return err
		}
		log.Printf("=== ОТЛАДКА SQLiteDB.SaveResult: Успешно обновлен результат выражения %s на %f", exprID, finalResult)
	} else {
		log.Printf("=== ОТЛАДКА SQLiteDB.SaveResult: Для выражения %s остались невыполненные задачи, не обновляем результат", exprID)
	}
	
	return nil
}

// GetResult возвращает результат задачи по ID
func (db *SQLiteDB) GetResult(taskID int) (float64, error) {
	var result float64
	err := db.db.QueryRow("SELECT result FROM expressions WHERE id = ?", taskID).Scan(&result)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("результат для задачи %d не найден", taskID)
		}
		return 0, err
	}
	return result, nil
}

// GetResultsByExprID возвращает все результаты для выражения
func (db *SQLiteDB) GetResultsByExprID(exprID string) (map[int]float64, error) {
	rows, err := db.db.Query(`
		SELECT task_id, result 
		FROM results 
		WHERE expression_id = ?
		ORDER BY created_at DESC`, exprID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[int]float64)
	for rows.Next() {
		var taskID int
		var result float64
		if err := rows.Scan(&taskID, &result); err != nil {
			return nil, err
		}
		results[taskID] = result
	}

	return results, nil
}

// GetLastResultByExprID возвращает последний результат для выражения
func (db *SQLiteDB) GetLastResultByExprID(exprID string) (*models.TaskResult, error) {
	result := &models.TaskResult{}
	err := db.db.QueryRow(`
		SELECT task_id, result 
		FROM results 
		WHERE expression_id = ?
		ORDER BY created_at DESC
		LIMIT 1`, exprID).Scan(&result.ID, &result.Result)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return result, nil
}
