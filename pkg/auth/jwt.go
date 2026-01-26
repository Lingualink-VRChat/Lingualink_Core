// jwt.go contains JWT authentication.
package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/sirupsen/logrus"
)

// JWTAuthenticator JWT认证器
type JWTAuthenticator struct {
	secret []byte
	logger *logrus.Logger
}

// NewJWTAuthenticator 创建JWT认证器
func NewJWTAuthenticator(config map[string]interface{}, logger *logrus.Logger) *JWTAuthenticator {
	secret := "default-secret-key"
	if s, ok := config["secret"].(string); ok {
		secret = s
	}
	return &JWTAuthenticator{
		secret: []byte(secret),
		logger: logger,
	}
}

// Authenticate 认证
func (auth *JWTAuthenticator) Authenticate(ctx context.Context, credentials Credentials) (*Identity, error) {
	tokenString := credentials.Token
	if strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return auth.secret, nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return &Identity{
			ID:          getString(claims, "sub", ""),
			ExternalID:  getString(claims, "user_id", ""),
			Type:        IdentityType(getString(claims, "type", "user")),
			Permissions: parsePermissions(claims["permissions"]),
			Metadata:    claims,
		}, nil
	}

	return nil, ErrInvalidToken
}

// GetType 获取认证器类型
func (auth *JWTAuthenticator) GetType() string {
	return "jwt"
}

func getString(claims jwt.MapClaims, key, defaultValue string) string {
	if v, ok := claims[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}

func parsePermissions(perms interface{}) []Permission {
	var permissions []Permission
	if permSlice, ok := perms.([]interface{}); ok {
		for _, p := range permSlice {
			if perm, ok := p.(string); ok {
				permissions = append(permissions, Permission(perm))
			}
		}
	}
	return permissions
}
