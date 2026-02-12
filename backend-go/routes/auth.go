package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Fexaop/mdp-ir-car-parking/query"
	"golang.org/x/oauth2"
)

type AuthRoutes struct {
	GoogleConfig   *oauth2.Config
	UserQueries    UserQueriesInterface
	AuthMiddleware AuthMiddlewareInterface
}

type UserQueriesInterface interface {
	CreateOrIgnoreUser(googleID, email string) error
	CreateOrUpdateUser(googleID, email, name, picture string) error
	GetUserByEmail(email string) (query.UserInfo, error)
	GetUserByGoogleID(googleID string) (query.UserInfo, error)
}

type AuthMiddlewareInterface interface {
	GenerateJWT(uid string) (string, error)
	GetUserIDFromRequest(r *http.Request) (string, error)
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type MobileAuthRequest struct {
	IDToken     string `json:"id_token"`
	AccessToken string `json:"access_token"`
}

type LoginResponse struct {
	Token string         `json:"token"`
	User  query.UserInfo `json:"user"`
}

func NewAuthRoutes(googleConfig *oauth2.Config, userQueries UserQueriesInterface, authMiddleware AuthMiddlewareInterface) *AuthRoutes {
	return &AuthRoutes{
		GoogleConfig:   googleConfig,
		UserQueries:    userQueries,
		AuthMiddleware: authMiddleware,
	}
}

// Google OAuth login handler
func (ar *AuthRoutes) LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Redirect to Google OAuth
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
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	json.NewDecoder(resp.Body).Decode(&user)

	err = ar.UserQueries.CreateOrUpdateUser(user.ID, user.Email, user.Name, user.Picture)
	if err != nil {
		http.Error(w, "Database error", 500)
		return
	}

	jwtToken, err := ar.AuthMiddleware.GenerateJWT(user.ID)
	if err != nil {
		http.Error(w, "Token generation failed", 500)
		return
	}

	// Redirect back to the app with token as URL parameter
	// For dev: localhost:1420, for production: the app handles it internally
	frontendURL := "http://localhost:1420"
	redirectURL := fmt.Sprintf("%s/?token=%s", frontendURL, jwtToken)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// MobileAuthHandler handles authentication from mobile Google Auth
func (ar *AuthRoutes) MobileAuthHandler(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req MobileAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Fetch user info from Google using the access token
	client := &http.Client{}
	userInfoReq, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	userInfoReq.Header.Set("Authorization", "Bearer "+req.AccessToken)

	resp, err := client.Do(userInfoReq)
	if err != nil {
		http.Error(w, "Failed to fetch user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Invalid access token", http.StatusUnauthorized)
		return
	}

	var user struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		http.Error(w, "Failed to parse user info", http.StatusInternalServerError)
		return
	}

	// Create or update user in database
	err = ar.UserQueries.CreateOrUpdateUser(user.ID, user.Email, user.Name, user.Picture)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Generate JWT token
	jwtToken, err := ar.AuthMiddleware.GenerateJWT(user.ID)
	if err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}

	// Return the JWT token
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": jwtToken,
	})
}

func (ar *AuthRoutes) ProtectedHandler(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	userID, err := ar.AuthMiddleware.GetUserIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Fetch user from database
	user, err := ar.UserQueries.GetUserByGoogleID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}