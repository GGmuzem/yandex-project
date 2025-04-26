package auth

import (
	"context"

	"github.com/GGmuzem/yandex-project/pkg/models"
)

type contextKey string

const userContextKey contextKey = "user"

// SetUserContext сохраняет пользователя в контексте
func SetUserContext(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// GetUserFromContext извлекает пользователя из контекста
func GetUserFromContext(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(userContextKey).(*models.User)
	return user, ok
}
