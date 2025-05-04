package auth

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/GGmuzem/yandex-project/internal/database"
	"github.com/GGmuzem/yandex-project/pkg/models"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	// Секретный ключ для подписи JWT токенов
	jwtSecret = []byte("super_secret_key_change_in_production")

	// Время жизни токена (1 час)
	tokenExpiration = time.Hour * 1

	// Ошибки
	ErrInvalidCredentials = errors.New("неверный логин или пароль")
	ErrInvalidToken       = errors.New("неверный или истекший токен")
	ErrUserExists         = errors.New("пользователь с таким логином уже существует")
)

// Claims структура для JWT-токена
type Claims struct {
	UserID int    `json:"user_id"`
	Login  string `json:"login"`
	jwt.RegisteredClaims
}

// GenerateToken создает JWT токен для пользователя
func GenerateToken(user *models.User) (string, error) {
	// Устанавливаем срок действия токена
	expirationTime := time.Now().Add(tokenExpiration)

	// Создаем claims с данными пользователя
	claims := &Claims{
		UserID: user.ID,
		Login:  user.Login,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   fmt.Sprintf("%d", user.ID),
		},
	}

	// Создаем токен с claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Подписываем токен секретным ключом
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken проверяет и валидирует JWT токен
func ValidateToken(tokenString string) (*Claims, error) {
	// Парсим токен
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Проверяем метод подписи
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("неожиданный метод подписи: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	// Проверяем и возвращаем claims
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// ExtractTokenFromRequest извлекает токен из HTTP-запроса
func ExtractTokenFromRequest(r *http.Request) string {
	// Получаем токен из заголовка Authorization
	bearerToken := r.Header.Get("Authorization")

	// Проверяем формат Bearer token
	if len(bearerToken) > 7 && strings.ToUpper(bearerToken[0:7]) == "BEARER " {
		return bearerToken[7:]
	}

	return ""
}

// RegisterUser регистрирует нового пользователя
func RegisterUser(db database.Database, req *models.RegisterRequest) error {
	// Проверяем, существует ли пользователь
	exists, err := db.UserExists(req.Login)
	if err != nil {
		return err
	}
	if exists {
		return ErrUserExists
	}

	// Создаем пользователя
	user := &models.User{
		Login:    req.Login,
		Password: req.Password,
	}

	_, err = db.CreateUser(user)
	return err
}

// LoginUser аутентифицирует пользователя и возвращает JWT токен
func LoginUser(db database.Database, req *models.LoginRequest) (string, error) {
	log.Printf("LoginUser: Попытка входа пользователя %s", req.Login)

	// Получаем пользователя по логину
	user, err := db.GetUserByLogin(req.Login)
	if err != nil {
		log.Printf("LoginUser: Пользователь %s не найден: %v", req.Login, err)
		return "", ErrInvalidCredentials
	}
	log.Printf("LoginUser: Пользователь %s найден в базе", req.Login)

	// Проверяем пароль
	log.Printf("LoginUser: Проверка пароля для пользователя %s", req.Login)
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		log.Printf("LoginUser: Неверный пароль для пользователя %s: %v", req.Login, err)
		return "", ErrInvalidCredentials
	}
	log.Printf("LoginUser: Пароль проверен успешно для пользователя %s", req.Login)

	// Генерируем токен
	log.Printf("LoginUser: Генерация JWT токена для пользователя %s", req.Login)
	token, err := GenerateToken(user)
	if err != nil {
		log.Printf("LoginUser: Ошибка генерации токена для пользователя %s: %v", req.Login, err)
		return "", err
	}
	log.Printf("LoginUser: JWT токен успешно создан для пользователя %s", req.Login)

	return token, nil
}

// AuthMiddleware middleware для проверки авторизации
func AuthMiddleware(db database.Database, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Извлекаем токен из запроса
		tokenString := ExtractTokenFromRequest(r)
		if tokenString == "" {
			http.Error(w, "Требуется авторизация", http.StatusUnauthorized)
			return
		}

		// Валидируем токен
		claims, err := ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Неверный токен", http.StatusUnauthorized)
			return
		}

		// Проверяем существование пользователя
		user, err := db.GetUserByLogin(claims.Login)
		if err != nil || user.ID != claims.UserID {
			http.Error(w, "Пользователь не найден", http.StatusUnauthorized)
			return
		}

		// Устанавливаем контекст пользователя
		r = r.WithContext(SetUserContext(r.Context(), user))

		// Передаем управление следующему обработчику
		next(w, r)
	}
}
