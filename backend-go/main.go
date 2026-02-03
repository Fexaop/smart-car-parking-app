package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Fexaop/mdp-ir-car-parking/config"
	"github.com/Fexaop/mdp-ir-car-parking/middleware"
	"github.com/Fexaop/mdp-ir-car-parking/query"
	"github.com/Fexaop/mdp-ir-car-parking/routes"
	"github.com/gorilla/mux"
)

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

	// Auth routes
	r.HandleFunc("/login", authRoutes.LoginHandler)
	r.HandleFunc("/callback", authRoutes.CallbackHandler)
	r.Handle("/protected", authMiddleware.AuthRequired(http.HandlerFunc(authRoutes.ProtectedHandler)))

	// Start server
	serverAddr := ":" + cfg.ServerPort
	fmt.Printf("Server running on http://localhost%s\n", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, r))
}
