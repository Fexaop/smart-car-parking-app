package middleware

import (
	"net/http"
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

func (am *AuthMiddleware) AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("Authorization")
		if tokenStr == "" {
			http.Error(w, "Missing token", 401)
			return
		}

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