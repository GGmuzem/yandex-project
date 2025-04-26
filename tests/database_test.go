package tests

import (
	"os"
	"testing"

	"github.com/GGmuzem/yandex-project/internal/database"
	"github.com/GGmuzem/yandex-project/pkg/models"
)

func TestDatabaseOperations(t *testing.T) {
	// Используем временный файл для тестов
	dbPath := "./test_db.sqlite"

	// Убедимся, что файл БД будет удален после тестов
	defer os.Remove(dbPath)

	// Создаем новую БД
	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("Не удалось создать базу данных: %v", err)
	}
	defer db.Close()

	// Выполняем миграции
	if err := db.MigrateDB(); err != nil {
		t.Fatalf("Не удалось выполнить миграции: %v", err)
	}

	// Тестируем создание пользователя
	t.Run("CreateUser", func(t *testing.T) {
		user := &models.User{
			Login:    "testuser",
			Password: "password123",
		}

		id, err := db.CreateUser(user)
		if err != nil {
			t.Fatalf("Не удалось создать пользователя: %v", err)
		}

		if id <= 0 {
			t.Errorf("Некорректный ID пользователя: %d", id)
		}
	})

	// Тестируем проверку существования пользователя
	t.Run("UserExists", func(t *testing.T) {
		exists, err := db.UserExists("testuser")
		if err != nil {
			t.Fatalf("Ошибка при проверке существования пользователя: %v", err)
		}

		if !exists {
			t.Error("Пользователь должен существовать")
		}

		exists, err = db.UserExists("nonexistentuser")
		if err != nil {
			t.Fatalf("Ошибка при проверке существования пользователя: %v", err)
		}

		if exists {
			t.Error("Пользователь не должен существовать")
		}
	})

	// Тестируем получение пользователя по логину
	t.Run("GetUserByLogin", func(t *testing.T) {
		user, err := db.GetUserByLogin("testuser")
		if err != nil {
			t.Fatalf("Не удалось получить пользователя: %v", err)
		}

		if user.Login != "testuser" {
			t.Errorf("Неверный логин пользователя: %s", user.Login)
		}

		if user.ID <= 0 {
			t.Errorf("Некорректный ID пользователя: %d", user.ID)
		}
	})

	// Тестируем сохранение выражения
	t.Run("SaveExpression", func(t *testing.T) {
		expr := &models.Expression{
			ID:     "test_expr_1",
			Status: "pending",
			UserID: 1,
		}

		err := db.SaveExpression(expr)
		if err != nil {
			t.Fatalf("Не удалось сохранить выражение: %v", err)
		}
	})

	// Тестируем получение выражения
	t.Run("GetExpression", func(t *testing.T) {
		expr, err := db.GetExpression("test_expr_1", 1)
		if err != nil {
			t.Fatalf("Не удалось получить выражение: %v", err)
		}

		if expr.ID != "test_expr_1" {
			t.Errorf("Неверный ID выражения: %s", expr.ID)
		}

		if expr.Status != "pending" {
			t.Errorf("Неверный статус выражения: %s", expr.Status)
		}
	})

	// Тестируем обновление статуса выражения
	t.Run("UpdateExpressionStatus", func(t *testing.T) {
		err := db.UpdateExpressionStatus("test_expr_1", "completed", 42.0)
		if err != nil {
			t.Fatalf("Не удалось обновить статус выражения: %v", err)
		}

		expr, err := db.GetExpression("test_expr_1", 1)
		if err != nil {
			t.Fatalf("Не удалось получить выражение: %v", err)
		}

		if expr.Status != "completed" {
			t.Errorf("Статус не обновился: %s", expr.Status)
		}

		if expr.Result != 42.0 {
			t.Errorf("Результат не обновился: %f", expr.Result)
		}
	})

	// Тестируем список выражений
	t.Run("GetExpressions", func(t *testing.T) {
		// Добавим еще одно выражение
		expr2 := &models.Expression{
			ID:     "test_expr_2",
			Status: "pending",
			UserID: 1,
		}

		err := db.SaveExpression(expr2)
		if err != nil {
			t.Fatalf("Не удалось сохранить второе выражение: %v", err)
		}

		expressions, err := db.GetExpressions(1)
		if err != nil {
			t.Fatalf("Не удалось получить список выражений: %v", err)
		}

		if len(expressions) != 2 {
			t.Errorf("Неверное количество выражений: %d, ожидалось: 2", len(expressions))
		}
	})

	// Тестируем сохранение результата
	t.Run("SaveResult", func(t *testing.T) {
		err := db.SaveResult(1, 10.5, "test_expr_1")
		if err != nil {
			t.Fatalf("Не удалось сохранить результат: %v", err)
		}
	})

	// Тестируем получение результата
	t.Run("GetResult", func(t *testing.T) {
		result, err := db.GetResult(1)
		if err != nil {
			t.Fatalf("Не удалось получить результат: %v", err)
		}

		if result != 10.5 {
			t.Errorf("Неверный результат: %f, ожидалось: 10.5", result)
		}
	})

	// Тестируем получение результатов по ID выражения
	t.Run("GetResultsByExprID", func(t *testing.T) {
		// Добавим еще один результат
		err := db.SaveResult(2, 20.5, "test_expr_1")
		if err != nil {
			t.Fatalf("Не удалось сохранить второй результат: %v", err)
		}

		results, err := db.GetResultsByExprID("test_expr_1")
		if err != nil {
			t.Fatalf("Не удалось получить результаты: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Неверное количество результатов: %d, ожидалось: 2", len(results))
		}

		if results[1] != 10.5 {
			t.Errorf("Неверный результат для задачи 1: %f, ожидалось: 10.5", results[1])
		}

		if results[2] != 20.5 {
			t.Errorf("Неверный результат для задачи 2: %f, ожидалось: 20.5", results[2])
		}
	})
}
