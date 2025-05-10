package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

// JWT Claims для аутентификации
type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Секретный ключ для JWT токенов (в реальном приложении использовали бы переменную окружения)
var jwtSecret = []byte("your-secret-key-here")

// Промежуточное ПО для проверки JWT токена
func JWTMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Получаем токен из заголовка Authorization или из cookie
		tokenString := r.Header.Get("Authorization")
		
		// Если токена нет в заголовке, проверяем в cookie
		if tokenString == "" {
			// Проверяем cookie
			cookie, err := r.Cookie("jwt")
			if err != nil {
				log.Printf("Токен не найден ни в заголовках, ни в cookie: %v", err)
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "Требуется авторизация"})
				return
			}
			tokenString = cookie.Value
		} else if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			// Убираем префикс "Bearer "
			tokenString = tokenString[7:]
		}

		// Парсим и проверяем токен
		claims := &Claims{}
		
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			// Проверяем метод подписи
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			log.Printf("Ошибка валидации JWT токена: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Недействительный токен"})
			return
		}

		// Проверяем, существует ли пользователь в БД
		_, err = dbManager.GetUserByID(claims.UserID)
		if err != nil {
			log.Printf("Пользователь с ID %d не найден в БД: %v", claims.UserID, err)
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Недействительный пользователь"})
			return
		}

		// Добавляем информацию о пользователе в контекст запроса
		ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "username", claims.Username)

		log.Printf("Пользователь %s (ID: %d) успешно авторизован", claims.Username, claims.UserID)

		// Вызываем следующий обработчик с обновленным контекстом
		next(w, r.WithContext(ctx))
	}
}

// Обработчик для регистрации пользователей
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Декодируем тело запроса
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&credentials); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error": "Некорректный запрос"}`)
		return
	}

	// Проверяем, что имя пользователя и пароль не пусты
	if credentials.Username == "" || credentials.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error": "Имя пользователя и пароль не могут быть пустыми"}`)
		return
	}

	// Хэшируем пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(credentials.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Ошибка при хэшировании пароля: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error": "Ошибка при регистрации"}`)
		return
	}

	// Сохраняем пользователя в БД
	userID, err := dbManager.SaveUser(credentials.Username, string(hashedPassword))
	if err != nil {
		log.Printf("Ошибка при сохранении пользователя: %v", err)
		
		if strings.Contains(err.Error(), "уже существует") {
			w.WriteHeader(http.StatusConflict)
			fmt.Fprintf(w, `{"error": "Пользователь с таким именем уже существует"}`)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error": "Ошибка при регистрации"}`)
		}
		return
	}
	
	// Успешная регистрация
	log.Printf("Пользователь %s зарегистрирован с ID %d", credentials.Username, userID)
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"message": "Пользователь успешно зарегистрирован"}`)
}

// Обработчик для входа пользователей
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Декодируем тело запроса
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&credentials); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error": "Некорректный запрос"}`)
		return
	}

	// Получаем пользователя из БД
	user, err := dbManager.GetUserByUsername(credentials.Username)
	if err != nil {
		log.Printf("Ошибка при поиске пользователя: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"error": "Неверное имя пользователя или пароль"}`)
		return
	}

	// Проверяем пароль
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(credentials.Password)); err != nil {
		log.Printf("Неверный пароль для пользователя %s: %v", credentials.Username, err)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"error": "Неверное имя пользователя или пароль"}`)
		return
	}

	// Создаем JWT токен с временем истечения 24 часа
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
	}

	// Добавляем стандартные claims для поддержки времени истечения
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
	}

	// Создаем токен
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Printf("Ошибка при создании JWT токена: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Ошибка авторизации"})
		return
	}

	// Устанавливаем токен в cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    tokenString,
		Expires:  expirationTime,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// Возвращаем токен и в JSON ответе тоже
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":    tokenString,
		"user_id":  user.ID,
		"username": user.Username,
		"expires":  expirationTime.Format(time.RFC3339),
	})
}

// Обработчик для главной страницы
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "index.html", nil)
}

// Обработчик для страницы авторизации
func AuthHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "auth.html", nil)
}

// Обработчик для страницы истории вычислений
func HistoryHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "history.html", nil)
}

// Обработчик для страницы деталей выражения
func ExpressionDetailsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	exprID := vars["id"]
	
	renderTemplate(w, "expression-details.html", map[string]interface{}{
		"ExprID": exprID,
	})
}

// Обработчик для страницы справки
func HelpHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "help.html", nil)
}

// Вспомогательная функция для отображения шаблонов
func renderTemplate(w http.ResponseWriter, templateName string, data interface{}) {
	templateFilePath := "./web/templates/" + templateName
	log.Printf("Загрузка шаблона: %s", templateFilePath)
	
	tmpl, err := template.ParseFiles(templateFilePath)
	if err != nil {
		log.Printf("Ошибка при загрузке шаблона: %v", err)
		http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, data)
}

// StartServer запускает HTTP-сервер с API и веб-интерфейсом
func StartServer() {
	// Инициализируем менеджер базы данных
	var err error
	dbManager, err = NewDBManager()
	if err != nil {
		log.Fatalf("Ошибка инициализации базы данных: %v", err)
	}

	router := mux.NewRouter()

	// API-эндпоинты для вычислений
	router.HandleFunc("/api/calculate", JWTMiddleware(CalculateHandler)).Methods("POST")
	router.HandleFunc("/api/expression/{id}", JWTMiddleware(GetExpressionHandler)).Methods("GET")
	
	// Маршрут для внутреннего API (для агентов)
	router.HandleFunc("/internal/task", TaskHandler).Methods("GET", "POST")

	// API-эндпоинты для пользовательской истории
	router.HandleFunc("/api/expressions", JWTMiddleware(GetUserExpressionsHandler)).Methods("GET")
	router.HandleFunc("/api/expression/{id}/tasks", JWTMiddleware(GetExpressionTasksHandler)).Methods("GET")

	// Аутентификация
	router.HandleFunc("/api/register", RegisterHandler).Methods("POST")
	router.HandleFunc("/api/login", LoginHandler).Methods("POST")
	
	// Обработчики для веб-страниц
	router.HandleFunc("/", IndexHandler).Methods("GET")
	router.HandleFunc("/auth", AuthHandler).Methods("GET")
	router.HandleFunc("/history", HistoryHandler).Methods("GET")
	router.HandleFunc("/expression/{id}", ExpressionDetailsHandler).Methods("GET")
	router.HandleFunc("/help", HelpHandler).Methods("GET")
	
	// Статические файлы
	fileServer := http.FileServer(http.Dir("./web/static"))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fileServer))

	// Запуск сервера
	port := "8081"
	log.Printf("Сервер запущен на порту %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
