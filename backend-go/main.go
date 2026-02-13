package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Fexaop/mdp-ir-car-parking/bluetooth"
	"github.com/Fexaop/mdp-ir-car-parking/config"
	"github.com/Fexaop/mdp-ir-car-parking/middleware"
	"github.com/Fexaop/mdp-ir-car-parking/parking"
	"github.com/Fexaop/mdp-ir-car-parking/query"
	"github.com/Fexaop/mdp-ir-car-parking/routes"
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

	// Initialize Bluetooth bridge using Python subprocess
	pythonScript := os.Getenv("PYTHON_BRIDGE_SCRIPT")
	if pythonScript == "" {
		pythonScript = "../bluetooth_bridge.py" // Default path
	}
	
	handler := bluetooth.NewParkingMessageHandler("http://localhost:" + cfg.ServerPort)
	btBridge := bluetooth.NewBluetoothBridge(handler)
	
	// Attempt to start Python bridge (non-blocking)
	go func() {
		log.Printf("Starting Python Bluetooth bridge: %s\n", pythonScript)
		if err := btBridge.StartPythonBridge(pythonScript); err != nil {
			log.Printf("Failed to start Bluetooth bridge: %v\n", err)
			log.Println("Parking system will run without Bluetooth integration")
		}
	}()
	
	// Give Python bridge time to initialize
	time.Sleep(2 * time.Second)
	
	// Set the Bluetooth bridge in parking package
	parking.SetBluetoothBridge(btBridge)

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
	r.HandleFunc("/auth/mobile", authRoutes.MobileAuthHandler).Methods("POST", "OPTIONS")
	r.Handle("/protected", authMiddleware.AuthRequired(http.HandlerFunc(authRoutes.ProtectedHandler))).Methods("GET", "OPTIONS")

	// Apply CORS middleware
	httpHandler := corsMiddleware(r)

	// Start server
	serverAddr := ":" + cfg.ServerPort
	fmt.Printf("Server running on http://localhost%s\n", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, httpHandler))
}
