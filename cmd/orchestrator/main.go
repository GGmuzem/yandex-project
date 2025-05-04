package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/GGmuzem/yandex-project/internal/database"
	"github.com/GGmuzem/yandex-project/internal/orchestrator"
	"github.com/joho/godotenv"
)

// Упрощенная точка входа для запуска сервера
func main() {
	// Добавляем флаг для указания порта
	var port string
	flag.StringVar(&port, "port", "", "PORT для HTTP сервера")
	flag.Parse()

	// Загрузка переменных окружения из .env файла, если он существует
	_ = godotenv.Load()

	// Логируем версию Go и статус CGO
	log.Printf("Go Version: %s", runtime.Version())
	log.Printf("CGO Enabled: %t", false) // Изменяем на false, так как не используем CGO

	// Настройка веб-интерфейса
	// Получаем пути к директориям из переменных окружения или используем значения по умолчанию
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "./web/static"
	}

	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "./web/templates"
	}

	// Проверяем существование путей
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		log.Printf("Директория статических файлов не найдена: %s", staticDir)
	} else {
		log.Printf("Директория статических файлов найдена: %s", staticDir)
	}

	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		log.Printf("Директория шаблонов не найдена: %s", templateDir)
	} else {
		log.Printf("Директория шаблонов найдена: %s", templateDir)
	}

	// Преобразуем пути к абсолютным
	staticDirAbs, err := filepath.Abs(staticDir)
	if err != nil {
		log.Printf("Ошибка получения абсолютного пути к статическим файлам: %v", err)
		staticDirAbs = staticDir
	}

	templateDirAbs, err := filepath.Abs(templateDir)
	if err != nil {
		log.Printf("Ошибка получения абсолютного пути к шаблонам: %v", err)
		templateDirAbs = templateDir
	}

	log.Printf("Абсолютный путь к статическим файлам: %s", staticDirAbs)
	log.Printf("Абсолютный путь к шаблонам: %s", templateDirAbs)

	// Инициализируем веб-обработчик
	webHandler := orchestrator.NewWebHandler(staticDirAbs, templateDirAbs)

	// Настраиваем маршруты для веб-интерфейса
	webHandler.SetupWebRoutes()

	// Создаем базу данных
	dbPath := "./data/calculator.db"
	db, err := database.New(dbPath)
	if err != nil {
		log.Fatalf("Ошибка создания базы данных: %v", err)
	}
	defer db.Close()

	// Выполняем миграции базы данных
	if err := db.MigrateDB(); err != nil {
		log.Fatalf("Ошибка миграции базы данных: %v", err)
	}

	// Создаем мьютексы для синхронизации
	var mu sync.Mutex
	var tasksMutex sync.Mutex

	// Запускаем gRPC сервер для обработки задач
	if err := orchestrator.StartGRPCServer(db, &mu, &tasksMutex); err != nil {
		log.Fatalf("Ошибка запуска gRPC сервера: %v", err)
	}

	// Инициализируем менеджер задач
	orchestrator.InitTaskManager()

	// Создаем обработчики аутентификации
	authHandlers := orchestrator.NewAuthHandlers(db)

	// Настраиваем маршруты API
	// Аутентификация
	http.HandleFunc("/api/v1/register", authHandlers.RegisterHandler)
	http.HandleFunc("/api/v1/login", authHandlers.LoginHandler)

	// Выражения
	http.HandleFunc("/api/v1/calculate", authHandlers.AuthMiddleware(authHandlers.CalculateWithAuthHandler))
	http.HandleFunc("/api/v1/expressions", authHandlers.AuthMiddleware(authHandlers.ListExpressionsWithAuthHandler))
	http.HandleFunc("/api/v1/expressions/", authHandlers.AuthMiddleware(authHandlers.GetExpressionWithAuthHandler))

	// Внутренний API для агентов
	http.HandleFunc("/internal/task", orchestrator.TaskHandler)

	// Проверка статуса сервера
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Запускаем HTTP сервер
	serverAddr := ":8080"

	// Используем порт из флага командной строки или из переменной окружения
	if port != "" {
		serverAddr = ":" + port
	} else if envPort := os.Getenv("HTTP_PORT"); envPort != "" {
		serverAddr = ":" + envPort
	}

	log.Printf("HTTP сервер запущен на http://localhost%s", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, nil))
}
