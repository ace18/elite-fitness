package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL  string
	JWTSecret    string
	Port         string
	FrontendURL  string
	AppEnv       string
	AnthropicKey string
	DeepSeekKey  string
	ResendAPIKey string

	// PlanProvider sceglie chi genera i piani: "anthropic" (default) o
	// "deepseek". Esplicito e non dedotto dalla chiave presente: con due chiavi
	// configurate, indovinare significherebbe cambiare fornitore senza che
	// nessuno l'abbia chiesto.
	PlanProvider string
	EmailFrom    string

	// TrustProxyHeaders — attivare SOLO se il backend sta dietro un proxy che
	// riscrive lui X-Forwarded-For. Se è false ci si fida di RemoteAddr.
	// Sbagliarlo rompe il rate limiting in un verso o nell'altro: acceso senza
	// proxy chiunque aggira il limite con un header falso; spento dietro un
	// proxy tutte le richieste arrivano dallo stesso IP e si bloccano a vicenda.
	TrustProxyHeaders bool

	// APIURL — URL pubblico di questo backend, serve a comporre i redirect_uri
	// assoluti che i provider OAuth richiedono e che vanno registrati identici
	// nelle rispettive console.
	APIURL string

	// StaticDir — cartella con il frontend compilato da servire dallo stesso
	// binario. Vuota in sviluppo (ci pensa vite su :5173); in produzione è
	// quello che rende l'app una sola origine.
	StaticDir string

	GoogleClientID     string
	GoogleClientSecret string

	AppleTeamID     string
	AppleServicesID string
	AppleKeyID      string
	ApplePrivateKey string
}

// OAuthRedirectURL costruisce il redirect_uri per un provider. Deve combaciare
// carattere per carattere con quello registrato dal provider.
func (c *Config) OAuthRedirectURL(provider string) string {
	return strings.TrimRight(c.APIURL, "/") + "/api/auth/oauth/" + provider + "/callback"
}

func (c *Config) GoogleConfigured() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != ""
}

func (c *Config) AppleConfigured() bool {
	return c.AppleTeamID != "" && c.AppleServicesID != "" && c.AppleKeyID != "" && c.ApplePrivateKey != ""
}

func Load() *Config {
	_ = godotenv.Load()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "development"
	}
	// onboarding@resend.dev è il mittente di prova di Resend: non richiede un
	// dominio verificato ma consegna solo all'email del titolare dell'account.
	emailFrom := os.Getenv("EMAIL_FROM")
	if emailFrom == "" {
		emailFrom = "ELITE <onboarding@resend.dev>"
	}
	planProvider := strings.ToLower(strings.TrimSpace(os.Getenv("PLAN_PROVIDER")))
	if planProvider == "" {
		planProvider = "anthropic"
	}

	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:" + port
	}
	return &Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		Port:         port,
		FrontendURL:  frontendURL,
		AppEnv:       appEnv,
		AnthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
		DeepSeekKey:  os.Getenv("DEEPSEEK_API_KEY"),
		ResendAPIKey: os.Getenv("RESEND_API_KEY"),
		PlanProvider: planProvider,
		EmailFrom:    emailFrom,

		TrustProxyHeaders:  os.Getenv("TRUST_PROXY_HEADERS") == "true",
		APIURL:             apiURL,
		StaticDir:          os.Getenv("STATIC_DIR"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		AppleTeamID:        os.Getenv("APPLE_TEAM_ID"),
		AppleServicesID:    os.Getenv("APPLE_SERVICES_ID"),
		AppleKeyID:         os.Getenv("APPLE_KEY_ID"),
		ApplePrivateKey:    os.Getenv("APPLE_PRIVATE_KEY"),
	}
}

func (c *Config) IsDev() bool { return c.AppEnv == "development" }
