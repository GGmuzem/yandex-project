package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/GGmuzem/yandex-project/internal/auth"
	"github.com/GGmuzem/yandex-project/internal/database"
	"github.com/GGmuzem/yandex-project/internal/orchestrator"
	"github.com/GGmuzem/yandex-project/pkg/models"
)

// IntegrationTest структура для интеграционных тестов
type IntegrationTest struct {
	DB        database.Database
	Server    *httptest.Server
	AuthToken string
	UserID    int
}

// Создает тестового пользователя и возвращает его ID и токен
func createTestUser(t *testing.T, serverURL string, loginSuffix string) (int, string) {
	// Регистрируем пользователя с уникальным логином
	registerURL := serverURL + "/api/v1/register"
	registerData := map[string]string{
		"login":    "testuser_" + loginSuffix,
		"password": "password123",
	}

	registerJSON, _ := json.Marshal(registerData)
	resp, err := http.Post(registerURL, "application/json", bytes.NewBuffer(registerJSON))
	if err != nil {
		t.Fatalf("Ошибка при регистрации пользователя: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Ошибка при регистрации пользователя, статус: %d", resp.StatusCode)
	}

	// Выполняем вход
	loginURL := serverURL + "/api/v1/login"
	loginJSON, _ := json.Marshal(registerData)
	resp, err = http.Post(loginURL, "application/json", bytes.NewBuffer(loginJSON))
	if err != nil {
		t.Fatalf("Ошибка при входе пользователя: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Ошибка при входе пользователя, статус: %d", resp.StatusCode)
	}

	var loginResponse struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&loginResponse); err != nil {
		t.Fatalf("Ошибка при декодировании ответа: %v", err)
	}

	// Получаем ID пользователя из токена
	claims, err := auth.ValidateToken(loginResponse.Token)
	if err != nil {
		t.Fatalf("Ошибка при валидации токена: %v", err)
	}

	return claims.UserID, loginResponse.Token
}

// SetupIntegrationTest подготавливает окружение для интеграционных тестов
func SetupIntegrationTest(t *testing.T, testName string) *IntegrationTest {
	// Используем временный файл для тестов с уникальным именем для каждого теста
	dbPath := fmt.Sprintf("./test_integration_%s.sqlite", testName)

	// Убедимся, что файл БД будет удален после тестов
	t.Cleanup(func() {
		os.Remove(dbPath)
	})

	// Создаем новую БД
	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("Не удалось создать базу данных: %v", err)
	}

	// Выполняем миграции
	if err := db.MigrateDB(); err != nil {
		t.Fatalf("Не удалось выполнить миграции: %v", err)
	}

	// Создаем обработчики аутентификации
	authHandlers := orchestrator.NewAuthHandlers(db)

	// Создаем маршрутизатор для HTTP запросов
	mux := http.NewServeMux()

	// Регистрируем пути API
	mux.HandleFunc("/api/v1/register", authHandlers.RegisterHandler)
	mux.HandleFunc("/api/v1/login", authHandlers.LoginHandler)
	mux.HandleFunc("/api/v1/calculate", authHandlers.AuthMiddleware(authHandlers.CalculateWithAuthHandler))
	mux.HandleFunc("/api/v1/expressions", authHandlers.AuthMiddleware(authHandlers.ListExpressionsWithAuthHandler))
	mux.HandleFunc("/api/v1/expressions/", authHandlers.AuthMiddleware(authHandlers.GetExpressionWithAuthHandler))
	mux.HandleFunc("/internal/task", orchestrator.TaskHandler)

	// Запускаем тестовый сервер
	server := httptest.NewServer(mux)

	// Создаем тестового пользователя с уникальным логином для каждого теста
	userID, token := createTestUser(t, server.URL, testName)

	return &IntegrationTest{
		DB:        db,
		Server:    server,
		AuthToken: token,
		UserID:    userID,
	}
}

// TestFullCalculationProcess тестирует полный процесс вычисления выражений
func TestFullCalculationProcess(t *testing.T) {
	integrationTest := SetupIntegrationTest(t, "calc")
	defer integrationTest.Server.Close()

	// Отправляем выражение на вычисление
	t.Run("CalculateExpression", func(t *testing.T) {
		// Подготавливаем запрос
		calculateURL := integrationTest.Server.URL + "/api/v1/calculate"
		expressionData := map[string]string{
			"expression": "2+2*2",
		}

		expressionJSON, _ := json.Marshal(expressionData)
		req, _ := http.NewRequest("POST", calculateURL, bytes.NewBuffer(expressionJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+integrationTest.AuthToken)

		// Отправляем запрос
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Ошибка при отправке запроса на вычисление: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Ошибка при вычислении выражения, статус: %d", resp.StatusCode)
		}

		// Декодируем ответ
		var calcResponse struct {
			ID string `json:"id"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&calcResponse); err != nil {
			t.Fatalf("Ошибка при декодировании ответа: %v", err)
		}

		if calcResponse.ID == "" {
			t.Fatal("ID выражения не должен быть пустым")
		}

		// Запоминаем ID выражения для следующих тестов
		expressionID := calcResponse.ID

		// Проверяем статус выражения (должен быть сначала "pending")
		expr, err := integrationTest.DB.GetExpression(expressionID, integrationTest.UserID)
		if err != nil {
			t.Fatalf("Не удалось получить выражение: %v", err)
		}

		if expr.Status != "pending" {
			t.Errorf("Неверный начальный статус выражения: %s, ожидалось: 'pending'", expr.Status)
		}

		// Ждем некоторое время, чтобы выражение успело обработаться
		// В реальном тесте нужно имитировать работу агента, но для простоты просто подождем
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Периодически проверяем статус выражения
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Проверяем статус выражения
				expr, err := integrationTest.DB.GetExpression(expressionID, integrationTest.UserID)
				if err != nil {
					t.Fatalf("Не удалось получить выражение: %v", err)
				}

				// Если статус изменился на "completed", тест пройден
				if expr.Status == "completed" {
					// Проверяем результат (2+2*2 = 6)
					if expr.Result != 6.0 {
						t.Errorf("Неверный результат выражения: %f, ожидалось: 6.0", expr.Result)
					}
					return
				}

			case <-ctx.Done():
				t.Fatal("Тайм-аут при ожидании завершения вычисления")
				return
			}
		}
	})
}

// TestListExpressions тестирует получение списка выражений
func TestListExpressions(t *testing.T) {
	integrationTest := SetupIntegrationTest(t, "list")
	defer integrationTest.Server.Close()

	// Создаем несколько выражений для пользователя
	for i := 0; i < 3; i++ {
		expr := &models.Expression{
			ID:     fmt.Sprintf("test_expr_%d", i), // Используем fmt.Sprintf вместо string(i+48)
			Status: "completed",
			Result: float64(i * 10),
			UserID: integrationTest.UserID,
		}

		if err := integrationTest.DB.SaveExpression(expr); err != nil {
			t.Fatalf("Не удалось сохранить выражение: %v", err)
		}
	}

	// Получаем список выражений
	listURL := integrationTest.Server.URL + "/api/v1/expressions"
	req, _ := http.NewRequest("GET", listURL, nil)
	req.Header.Set("Authorization", "Bearer "+integrationTest.AuthToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Ошибка при запросе списка выражений: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Ошибка при запросе списка выражений, статус: %d", resp.StatusCode)
	}

	var listResponse struct {
		Expressions []models.Expression `json:"expressions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&listResponse); err != nil {
		t.Fatalf("Ошибка при декодировании ответа: %v", err)
	}

	// Проверяем, что список содержит все созданные выражения
	if len(listResponse.Expressions) < 3 {
		t.Errorf("Неверное количество выражений в списке: %d, ожидалось не менее 3", len(listResponse.Expressions))
	}
}
