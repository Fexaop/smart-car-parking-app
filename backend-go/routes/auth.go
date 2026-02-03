package routes

import (
	"context"
	"encoding/json"
	"net/http"

	"golang.org/x/oauth2"
)

type AuthRoutes struct {
	GoogleConfig   *oauth2.Config
	UserQueries    UserQueriesInterface
	AuthMiddleware AuthMiddlewareInterface
}

type UserQueriesInterface interface {
	CreateOrIgnoreUser(googleID, email string) error
}

type AuthMiddlewareInterface interface {
	GenerateJWT(uid string) (string, error)
}

func NewAuthRoutes(googleConfig *oauth2.Config, userQueries UserQueriesInterface, authMiddleware AuthMiddlewareInterface) *AuthRoutes {
	return &AuthRoutes{
		GoogleConfig:   googleConfig,
		UserQueries:    userQueries,
		AuthMiddleware: authMiddleware,
	}
}

func (ar *AuthRoutes) LoginHandler(w http.ResponseWriter, r *http.Request) {
	url := ar.GoogleConfig.AuthCodeURL("state-token")
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (ar *AuthRoutes) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")

	token, err := ar.GoogleConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "OAuth exchange failed", 500)
		return
	}

	client := ar.GoogleConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		http.Error(w, "User info failed", 500)
		return
	}
	defer resp.Body.Close()

	var user struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	json.NewDecoder(resp.Body).Decode(&user)

	err = ar.UserQueries.CreateOrIgnoreUser(user.ID, user.Email)
	if err != nil {
		http.Error(w, "Database error", 500)
		return
	}

	jwtToken, err := ar.AuthMiddleware.GenerateJWT(user.ID)
	if err != nil {
		http.Error(w, "Token generation failed", 500)
		return
	}

	// Return token as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": jwtToken,
	})
}

func (ar *AuthRoutes) ProtectedHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Authorized access success"))
}