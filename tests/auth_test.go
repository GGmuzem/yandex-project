package tests

import (
	"os"
	"testing"

	"github.com/GGmuzem/yandex-project/internal/auth"
	"github.com/GGmuzem/yandex-project/internal/database"
	"github.com/GGmuzem/yandex-project/pkg/models"
)

func TestAuthentication(t *testing.T) {
	// Используем временный файл для тестов
	dbPath := "./test_auth.sqlite"

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

	// Тестируем регистрацию пользователя
	t.Run("RegisterUser", func(t *testing.T) {
		req := &models.RegisterRequest{
			Login:    "testuser",
			Password: "password123",
		}

		err := auth.RegisterUser(db, req)
		if err != nil {
			t.Fatalf("Не удалось зарегистрировать пользователя: %v", err)
		}

		// Проверяем, что пользователь существует
		exists, err := db.UserExists("testuser")
		if err != nil {
			t.Fatalf("Ошибка при проверке существования пользователя: %v", err)
		}

		if !exists {
			t.Error("Пользователь должен существовать после регистрации")
		}

		// Проверяем дублирование
		err = auth.RegisterUser(db, req)
		if err != auth.ErrUserExists {
			t.Errorf("Ожидалась ошибка дублирования, получено: %v", err)
		}
	})

	// Тестируем вход пользователя
	t.Run("LoginUser", func(t *testing.T) {
		req := &models.LoginRequest{
			Login:    "testuser",
			Password: "password123",
		}

		token, err := auth.LoginUser(db, req)
		if err != nil {
			t.Fatalf("Не удалось войти: %v", err)
		}

		if token == "" {
			t.Error("Токен не должен быть пустым")
		}

		// Проверяем неверный пароль
		wrongReq := &models.LoginRequest{
			Login:    "testuser",
			Password: "wrongpassword",
		}

		_, err = auth.LoginUser(db, wrongReq)
		if err != auth.ErrInvalidCredentials {
			t.Errorf("Ожидалась ошибка неверных учетных данных, получено: %v", err)
		}
	})

	// Тестируем валидацию токена
	t.Run("ValidateToken", func(t *testing.T) {
		// Создаем пользователя и получаем токен
		user := &models.User{
			ID:    1,
			Login: "testuser",
		}

		token, err := auth.GenerateToken(user)
		if err != nil {
			t.Fatalf("Не удалось создать токен: %v", err)
		}

		// Валидируем токен
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Не удалось валидировать токен: %v", err)
		}

		if claims.UserID != user.ID {
			t.Errorf("Неверный ID пользователя в токене: %d, ожидалось: %d", claims.UserID, user.ID)
		}

		if claims.Login != user.Login {
			t.Errorf("Неверный логин пользователя в токене: %s, ожидалось: %s", claims.Login, user.Login)
		}

		// Валидируем некорректный токен
		_, err = auth.ValidateToken("invalid.token.string")
		if err == nil {
			t.Error("Должна быть ошибка при валидации некорректного токена")
		}
	})
}
