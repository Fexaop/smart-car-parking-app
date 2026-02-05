package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Fexaop/mdp-ir-car-parking/config"
	"github.com/Fexaop/mdp-ir-car-parking/middleware"
	"github.com/Fexaop/mdp-ir-car-parking/query"
	"github.com/Fexaop/mdp-ir-car-parking/routes"
	"github.com/Fexaop/mdp-ir-car-parking/parking"
	"github.com/gorilla/mux"
)

// CORS Middleware
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Load configuration from .env file
	cfg := config.LoadConfig()

	// Initialize database
	db, err := query.InitDB(cfg.DBPath)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// Initialize dependencies
	userQueries := query.NewUserQueries(db)
	authMiddleware := middleware.NewAuthMiddleware(cfg.JWTSecret)
	authRoutes := routes.NewAuthRoutes(cfg.GoogleOAuthConfig, userQueries, authMiddleware)

	// Setup routes
	r := mux.NewRouter()

    // Parking API routes
    parking.RegisterRoutes(r)

	// Auth routes
	r.HandleFunc("/login", authRoutes.LoginHandler).Methods("GET")
	r.HandleFunc("/callback", authRoutes.CallbackHandler).Methods("GET")
	r.Handle("/protected", authMiddleware.AuthRequired(http.HandlerFunc(authRoutes.ProtectedHandler))).Methods("GET", "OPTIONS")

	// Apply CORS middleware
	handler := corsMiddleware(r)

	// Start server
	serverAddr := ":" + cfg.ServerPort
	fmt.Printf("Server running on http://localhost%s\n", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, handler))
}
