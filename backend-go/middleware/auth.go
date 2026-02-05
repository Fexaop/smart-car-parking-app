package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthMiddleware struct {
	JWTSecret []byte
}

func NewAuthMiddleware(jwtSecret []byte) *AuthMiddleware {
	return &AuthMiddleware{JWTSecret: jwtSecret}
}

func (am *AuthMiddleware) GenerateJWT(uid string) (string, error) {
	claims := jwt.MapClaims{
		"uid": uid,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(am.JWTSecret)
}

func (am *AuthMiddleware) GetUserIDFromRequest(r *http.Request) (string, error) {
	tokenStr := r.Header.Get("Authorization")
	if tokenStr == "" {
		return "", errors.New("missing token")
	}

	// Remove "Bearer " prefix if present
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return am.JWTSecret, nil
	})

	if err != nil || !token.Valid {
		return "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid claims")
	}

	uid, ok := claims["uid"].(string)
	if !ok {
		return "", errors.New("invalid uid")
	}

	return uid, nil
}

func (am *AuthMiddleware) AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("Authorization")
		if tokenStr == "" {
			http.Error(w, "Missing token", 401)
			return
		}

		// Remove "Bearer " prefix if present
		tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return am.JWTSecret, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", 401)
			return
		}

		next.ServeHTTP(w, r)
	})
}