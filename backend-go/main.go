package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	gobetterauth "github.com/GoBetterAuth/go-better-auth/v2"
	gobetterauthconfig "github.com/GoBetterAuth/go-better-auth/v2/config"
	gobetterauthenv "github.com/GoBetterAuth/go-better-auth/v2/env"
	gobetterauthmodels "github.com/GoBetterAuth/go-better-auth/v2/models"
	oauth2plugin "github.com/GoBetterAuth/go-better-auth/v2/plugins/oauth2"
	oauth2plugintypes "github.com/GoBetterAuth/go-better-auth/v2/plugins/oauth2/types"
)

func main() {
	config := gobetterauthconfig.NewConfig(
		gobetterauthconfig.WithAppName("MDPIRCarParking"),
		gobetterauthconfig.WithBasePath("/api/auth"),
		gobetterauthconfig.WithBaseURL(getEnvOrDefault("BASE_URL", "http://localhost:8080")),
		gobetterauthconfig.WithDatabase(gobetterauthmodels.DatabaseConfig{
			Provider: "sqlite",
			URL:      getEnvOrDefault("DATABASE_URL", "./sqlite.db"),
		}),
		gobetterauthconfig.WithSecurity(gobetterauthmodels.SecurityConfig{
			// Configure CORS and Trusted Origins appropriately
			TrustedOrigins: []string{
				getEnvOrDefault("FRONTEND_URL", "http://localhost:3000"),
			},
			CORS: gobetterauthmodels.CORSConfig{
				AllowCredentials: true,
				AllowedOrigins: []string{
					getEnvOrDefault("FRONTEND_URL", "http://localhost:3000"),
				},
			},
		}),
	)

	auth := gobetterauth.New(&gobetterauth.AuthConfig{
		Config: config,
		Plugins: []gobetterauthmodels.Plugin{
			oauth2plugin.New(oauth2plugintypes.OAuth2PluginConfig{
				Enabled: true,
				Providers: map[string]oauth2plugintypes.ProviderConfig{
					"google": {
						Enabled:      true,
						ClientID:     os.Getenv(gobetterauthenv.EnvGoogleClientID),
						ClientSecret: os.Getenv(gobetterauthenv.EnvGoogleClientSecret),
						RedirectURL:  fmt.Sprintf("%s%s/oauth2/callback/google", config.BaseURL, config.BasePath),
					},
				},
			}),
		},
	})

	// Mount auth endpoints
	http.Handle("/api/auth/", auth.Handler())

	log.Println("Server starting on :8080")
	log.Printf("Google OAuth authorize URL: %s/api/auth/oauth2/authorize/google?redirect_to=<YOUR_REDIRECT_URL>", config.BaseURL)
	log.Printf("Google OAuth callback URL: %s/api/auth/oauth2/callback/google", config.BaseURL)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
