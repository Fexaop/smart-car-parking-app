package routes

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Fexaop/mdp-ir-car-parking/query"
	"golang.org/x/oauth2"
)

type AuthRoutes struct {
	GoogleConfig       *oauth2.Config
	GoogleMobileConfig *oauth2.Config
	UserQueries        UserQueriesInterface
	AuthMiddleware     AuthMiddlewareInterface
	mobileAuthMu       sync.Mutex
	mobileAuthSessions map[string]mobileAuthSession
}

const (
	mobileAuthStatePrefix = "mobile:"
	mobileAuthLinkTTL     = 10 * time.Minute
	mobileAuthOTPTTL      = 5 * time.Minute
	webCallbackPath       = "/callback"
	mobileCallbackPath    = "/auth/mobile/callback"
	frontendPort          = "1420"
)

type mobileAuthSession struct {
	OTP       string
	Token     string
	User      query.UserInfo
	OTPReady  bool
	ExpiresAt time.Time
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

type MobileLoginLinkResponse struct {
	RequestID string `json:"request_id"`
	LoginURL  string `json:"login_url"`
	ExpiresIn int    `json:"expires_in"`
}

type MobileOTPVerifyRequest struct {
	RequestID string `json:"request_id"`
	OTP       string `json:"otp"`
}

type LoginResponse struct {
	Token string         `json:"token"`
	User  query.UserInfo `json:"user"`
}

func NewAuthRoutes(googleConfig *oauth2.Config, googleMobileConfig *oauth2.Config, userQueries UserQueriesInterface, authMiddleware AuthMiddlewareInterface) *AuthRoutes {
	return &AuthRoutes{
		GoogleConfig:       googleConfig,
		GoogleMobileConfig: googleMobileConfig,
		UserQueries:        userQueries,
		AuthMiddleware:     authMiddleware,
		mobileAuthSessions: make(map[string]mobileAuthSession),
	}
}

func (ar *AuthRoutes) LoginHandler(w http.ResponseWriter, r *http.Request) {
	googleConfig := oauthConfigForRequest(ar.GoogleConfig, r, webCallbackPath)
	if googleConfig == nil {
		http.Error(w, "OAuth not configured", http.StatusInternalServerError)
		return
	}

	url := googleConfig.AuthCodeURL("state-token")
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (ar *AuthRoutes) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	googleConfig := oauthConfigForRequest(ar.GoogleConfig, r, webCallbackPath)
	if googleConfig == nil {
		http.Error(w, "OAuth not configured", http.StatusInternalServerError)
		return
	}

	code := r.URL.Query().Get("code")

	token, err := googleConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "OAuth exchange failed", 500)
		return
	}

	user, err := fetchGoogleUserInfo(googleConfig, token)
	if err != nil {
		http.Error(w, "User info failed", http.StatusInternalServerError)
		return
	}

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

	frontendURL := frontendBaseURL(r)
	redirectURL := fmt.Sprintf("%s/?token=%s", frontendURL, jwtToken)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (ar *AuthRoutes) MobileLinkHandler(w http.ResponseWriter, r *http.Request) {
	if ar.GoogleMobileConfig == nil {
		http.Error(w, "Mobile OAuth not configured", http.StatusInternalServerError)
		return
	}

	requestID, err := generateMobileRequestID()
	if err != nil {
		http.Error(w, "Failed to generate mobile auth link", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	ar.mobileAuthMu.Lock()
	ar.cleanupExpiredMobileAuthSessionsLocked(now)
	ar.mobileAuthSessions[requestID] = mobileAuthSession{ExpiresAt: now.Add(mobileAuthLinkTTL)}
	ar.mobileAuthMu.Unlock()

	loginURL := fmt.Sprintf("%s/auth/mobile/login?request_id=%s", requestBaseURL(r), requestID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MobileLoginLinkResponse{
		RequestID: requestID,
		LoginURL:  loginURL,
		ExpiresIn: int(mobileAuthLinkTTL.Seconds()),
	})
}

func (ar *AuthRoutes) MobileLoginHandler(w http.ResponseWriter, r *http.Request) {
	if ar.GoogleMobileConfig == nil {
		http.Error(w, "Mobile OAuth not configured", http.StatusInternalServerError)
		return
	}

	requestID := strings.TrimSpace(r.URL.Query().Get("request_id"))
	if requestID == "" {
		http.Error(w, "Missing request_id", http.StatusBadRequest)
		return
	}

	now := time.Now()
	ar.mobileAuthMu.Lock()
	ar.cleanupExpiredMobileAuthSessionsLocked(now)
	session, ok := ar.mobileAuthSessions[requestID]
	if !ok || session.ExpiresAt.Before(now) {
		ar.mobileAuthMu.Unlock()
		http.Error(w, "Invalid or expired mobile login request", http.StatusBadRequest)
		return
	}
	ar.mobileAuthMu.Unlock()

	state := mobileAuthStatePrefix + requestID
	googleConfig := oauthConfigForRequest(ar.GoogleMobileConfig, r, mobileCallbackPath)
	if googleConfig == nil {
		http.Error(w, "Mobile OAuth not configured", http.StatusInternalServerError)
		return
	}

	url := googleConfig.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (ar *AuthRoutes) MobileCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if ar.GoogleMobileConfig == nil {
		http.Error(w, "Mobile OAuth not configured", http.StatusInternalServerError)
		return
	}

	googleConfig := oauthConfigForRequest(ar.GoogleMobileConfig, r, mobileCallbackPath)
	if googleConfig == nil {
		http.Error(w, "Mobile OAuth not configured", http.StatusInternalServerError)
		return
	}

	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if !strings.HasPrefix(state, mobileAuthStatePrefix) {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	requestID := strings.TrimPrefix(state, mobileAuthStatePrefix)
	if requestID == "" {
		http.Error(w, "Missing request id", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	token, err := googleConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "OAuth exchange failed", 500)
		return
	}

	user, err := fetchGoogleUserInfo(googleConfig, token)
	if err != nil {
		http.Error(w, "User info failed", http.StatusInternalServerError)
		return
	}

	err = ar.UserQueries.CreateOrUpdateUser(user.ID, user.Email, user.Name, user.Picture)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	jwtToken, err := ar.AuthMiddleware.GenerateJWT(user.ID)
	if err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}

	otp, err := generateOneTimeCode()
	if err != nil {
		http.Error(w, "Failed to generate OTP", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	ar.mobileAuthMu.Lock()
	ar.cleanupExpiredMobileAuthSessionsLocked(now)
	session, ok := ar.mobileAuthSessions[requestID]
	if !ok || session.ExpiresAt.Before(now) {
		ar.mobileAuthMu.Unlock()
		http.Error(w, "Mobile login request expired", http.StatusBadRequest)
		return
	}
	session.OTP = otp
	session.Token = jwtToken
	session.User = user
	session.OTPReady = true
	session.ExpiresAt = now.Add(mobileAuthOTPTTL)
	ar.mobileAuthSessions[requestID] = session
	ar.mobileAuthMu.Unlock()

	if r.URL.Query().Get("raw") == "1" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"request_id": requestID,
			"otp":        otp,
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<!doctype html><html><head><meta charset='utf-8'><meta name='viewport' content='width=device-width, initial-scale=1'><title>Mobile Login OTP</title><style>body{font-family:Arial,sans-serif;background:#f4f5f7;color:#111;margin:0;padding:24px}main{max-width:420px;margin:0 auto;background:#fff;border-radius:12px;padding:20px;box-shadow:0 8px 24px rgba(0,0,0,0.12)}h1{font-size:24px;margin-top:0}p{line-height:1.5}.otp{font-size:44px;font-weight:700;letter-spacing:8px;text-align:center;margin:20px 0;padding:14px;border:2px solid #111;border-radius:10px;background:#f8fafc}.meta{font-size:14px;color:#555}</style></head><body><main><h1>Enter this OTP in the app</h1><p class='otp'>%s</p><p>This OTP expires in %d minutes.</p><p class='meta'>Request ID: %s</p></main></body></html>", otp, int(mobileAuthOTPTTL.Minutes()), requestID)
}

func (ar *AuthRoutes) MobileOTPVerifyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req MobileOTPVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.RequestID = strings.TrimSpace(req.RequestID)
	req.OTP = strings.TrimSpace(req.OTP)
	if req.RequestID == "" || req.OTP == "" {
		http.Error(w, "request_id and otp are required", http.StatusBadRequest)
		return
	}

	now := time.Now()
	ar.mobileAuthMu.Lock()
	ar.cleanupExpiredMobileAuthSessionsLocked(now)
	session, ok := ar.mobileAuthSessions[req.RequestID]
	if !ok || !session.OTPReady || session.ExpiresAt.Before(now) {
		if ok {
			delete(ar.mobileAuthSessions, req.RequestID)
		}
		ar.mobileAuthMu.Unlock()
		http.Error(w, "Invalid or expired OTP", http.StatusUnauthorized)
		return
	}

	if subtle.ConstantTimeCompare([]byte(session.OTP), []byte(req.OTP)) != 1 {
		ar.mobileAuthMu.Unlock()
		http.Error(w, "Invalid or expired OTP", http.StatusUnauthorized)
		return
	}

	delete(ar.mobileAuthSessions, req.RequestID)
	ar.mobileAuthMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{Token: session.Token, User: session.User})
}

func (ar *AuthRoutes) MobileAuthHandler(w http.ResponseWriter, r *http.Request) {
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

	err = ar.UserQueries.CreateOrUpdateUser(user.ID, user.Email, user.Name, user.Picture)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	jwtToken, err := ar.AuthMiddleware.GenerateJWT(user.ID)
	if err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": jwtToken,
	})
}

func (ar *AuthRoutes) ProtectedHandler(w http.ResponseWriter, r *http.Request) {
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

	user, err := ar.UserQueries.GetUserByGoogleID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (ar *AuthRoutes) cleanupExpiredMobileAuthSessionsLocked(now time.Time) {
	for requestID, session := range ar.mobileAuthSessions {
		if session.ExpiresAt.Before(now) {
			delete(ar.mobileAuthSessions, requestID)
		}
	}
}

func generateMobileRequestID() (string, error) {
	randomBytes := make([]byte, 18)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func generateOneTimeCode() (string, error) {
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	value := binary.BigEndian.Uint32(randomBytes) % 1000000
	return fmt.Sprintf("%06d", value), nil
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if forwardedProto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); r.TLS != nil || strings.EqualFold(forwardedProto, "https") {
		scheme = "https"
	}

	host := r.Host
	if forwardedHost := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0]); forwardedHost != "" {
		host = forwardedHost
	}

	if host == "" {
		return ""
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}

func oauthConfigForRequest(baseConfig *oauth2.Config, r *http.Request, callbackPath string) *oauth2.Config {
	if baseConfig == nil {
		return nil
	}

	config := *baseConfig
	if baseURL := requestBaseURL(r); baseURL != "" {
		config.RedirectURL = baseURL + callbackPath
	}

	return &config
}

func frontendBaseURL(r *http.Request) string {
	const fallback = "http://localhost:" + frontendPort

	baseURL := requestBaseURL(r)
	if baseURL == "" {
		return fallback
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" {
		return fallback
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return fallback
	}

	parsed.Host = net.JoinHostPort(hostname, frontendPort)
	return parsed.String()
}

func fetchGoogleUserInfo(oauthConfig *oauth2.Config, token *oauth2.Token) (query.UserInfo, error) {
	client := oauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return query.UserInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return query.UserInfo{}, fmt.Errorf("google user info request failed with status %d", resp.StatusCode)
	}

	var user query.UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return query.UserInfo{}, err
	}

	return user, nil
}
