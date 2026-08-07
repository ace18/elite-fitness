package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/elitecoach/backend/internal/config"
	"github.com/elitecoach/backend/internal/db"
	"github.com/elitecoach/backend/internal/handler"
	"github.com/elitecoach/backend/internal/middleware"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/elitecoach/backend/internal/service"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	cfg := config.Load()

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		if cfg.IsDev() {
			cfg.JWTSecret = "dev-secret-change-in-production-32ch"
			fmt.Println("[warn] using default dev JWT secret")
		} else {
			log.Fatal("JWT_SECRET is required in production")
		}
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	// repos
	userRepo := repository.NewUserRepo(pool)
	programRepo := repository.NewProgramRepo(pool)
	sessionRepo := repository.NewSessionRepo(pool)
	oauthStateRepo := repository.NewOAuthStateRepo(pool)

	// services
	var mailer service.Mailer
	if cfg.ResendAPIKey != "" {
		mailer = service.NewResendMailer(cfg.ResendAPIKey, cfg.EmailFrom)
		fmt.Printf("[mail] resend enabled, from %s\n", cfg.EmailFrom)
	} else if cfg.IsDev() {
		// Senza mailer il login resta completabile solo grazie al devToken.
		fmt.Println("[warn] RESEND_API_KEY unset — magic-link emails will not be sent")
	} else {
		// In produzione niente email significa che nessuno può autenticarsi:
		// meglio non partire affatto che accettare login che falliranno.
		log.Fatal("RESEND_API_KEY is required in production")
	}
	authSvc := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.IsDev(), mailer, cfg.FrontendURL)

	// Ogni provider entra nel registro solo se configurato: il frontend chiede
	// /api/auth/providers e disegna i bottoni di conseguenza.
	var providers []service.OAuthProvider
	if cfg.GoogleConfigured() {
		providers = append(providers, service.NewGoogleProvider(
			cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.OAuthRedirectURL("google")))
	}
	if cfg.AppleConfigured() {
		apple, err := service.NewAppleProvider(
			cfg.AppleTeamID, cfg.AppleServicesID, cfg.AppleKeyID, cfg.ApplePrivateKey,
			cfg.OAuthRedirectURL("apple"))
		if err != nil {
			// Chiave .p8 illeggibile: meglio saperlo all'avvio che al primo login.
			log.Fatalf("apple oauth: %v", err)
		}
		providers = append(providers, apple)
	}
	oauthRegistry := service.NewOAuthRegistry(providers...)
	if oauthRegistry.Enabled() {
		fmt.Printf("[oauth] enabled: %s\n", strings.Join(oauthRegistry.Names(), ", "))
	} else {
		fmt.Println("[oauth] no providers configured — magic link only")
	}
	// Potatura: senza questa magic_link_tokens e oauth_states crescono per
	// sempre, una riga per ogni tentativo di login.
	pruner := service.NewPruner(
		service.DefaultPruneInterval, service.DefaultPruneRetention,
		service.PruneTask{Name: "magic_link_tokens", Delete: userRepo.DeleteExpiredMagicLinksBefore},
		service.PruneTask{Name: "oauth_states", Delete: oauthStateRepo.DeleteExpiredBefore},
	)
	go pruner.Run(ctx)

	workoutSvc := service.NewWorkoutService(programRepo, sessionRepo)
	planner, err := newPlanner(cfg)
	if err != nil {
		log.Fatal(err)
	}
	aiSvc := service.NewAIService(planner, programRepo, pool)
	planJobs := service.NewPlanJobStore()

	// handlers
	authH := handler.NewAuthHandler(authSvc, userRepo)
	oauthH := handler.NewOAuthHandler(oauthRegistry, oauthStateRepo, userRepo, cfg.FrontendURL)
	programH := handler.NewProgramHandler(programRepo, aiSvc, planJobs)
	workoutH := handler.NewWorkoutHandler(workoutSvc)
	completer := service.NewProgramCompleter(programRepo, sessionRepo)
	sessionH := handler.NewSessionHandler(sessionRepo, completer)
	progressH := handler.NewProgressHandler(sessionRepo, programRepo)

	// router
	r := chi.NewRouter()
	// Il rate limiter delle magic link fa da chiave sull'IP del client, quindi
	// RemoteAddr dev'essere quello vero. Dietro Cloudflare l'unico header
	// attendibile è CF-Connecting-IP: X-Forwarded-For e True-Client-IP sono
	// falsificabili dal client (vedi middleware/realip.go). Da tenere spento se
	// il backend è esposto direttamente.
	if cfg.TrustProxyHeaders {
		r.Use(middleware.StripForwardedHeaders)
		r.Use(middleware.CloudflareIP)
		fmt.Println("[warn] TRUST_PROXY_HEADERS=true — CF-Connecting-IP is trusted; only correct behind Cloudflare")
	}
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	// Una sola origine (STATIC_DIR impostato) vuol dire nessuna richiesta
	// cross-origin da autorizzare: il CORS serve solo in sviluppo, dove vite
	// gira su una porta diversa.
	if cfg.StaticDir == "" {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{cfg.FrontendURL},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
			AllowCredentials: true,
		}))
	}

	// Health check: Coolify lo interroga per sapere se il container è vivo.
	// Volutamente senza toccare il DB — deve dire "il processo risponde", non
	// "tutta la pila è sana", altrimenti un blip di Postgres fa riavviare l'app.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// public auth routes
	r.Post("/api/auth/magic-link", authH.SendMagicLink)
	r.Get("/api/auth/verify", authH.Verify)
	r.Get("/api/auth/providers", oauthH.Providers)
	r.Get("/api/auth/oauth/{provider}/start", oauthH.Start)
	// Google torna in GET, Apple in POST form-encoded (response_mode=form_post).
	r.Get("/api/auth/oauth/{provider}/callback", oauthH.Callback)
	r.Post("/api/auth/oauth/{provider}/callback", oauthH.Callback)

	// protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(authSvc))

		r.Get("/api/auth/me", authH.Me)
		r.Get("/api/workout/today", workoutH.GetToday)
		r.Get("/api/program", programH.GetProgram)
		r.Post("/api/program", programH.SetProgram)
		r.Get("/api/plans", programH.GetPlans)
		r.Post("/api/plans/generate", programH.GeneratePlan)
		r.Get("/api/plans/generate/{jobId}", programH.GetPlanJob)
		r.Post("/api/sessions", sessionH.SaveSession)
		r.Get("/api/sessions/last", sessionH.GetLastSession)
		r.Get("/api/progress", progressH.GetProgress)
		r.Post("/api/progress/weight", progressH.LogWeight)
	})

	// Il frontend compilato, se c'è, sta sotto la stessa origine dell'API.
	// Va registrato per ultimo: è il catch-all di tutto ciò che non è una rotta
	// dichiarata sopra.
	if cfg.StaticDir != "" {
		if _, err := os.Stat(filepath.Join(cfg.StaticDir, "index.html")); err != nil {
			// Meglio non partire che servire 404 su ogni pagina finché qualcuno
			// non se ne accorge.
			log.Fatalf("STATIC_DIR=%s has no index.html: %v", cfg.StaticDir, err)
		}
		r.NotFound(handler.SPA(cfg.StaticDir).ServeHTTP)
		fmt.Printf("[static] serving SPA from %s\n", cfg.StaticDir)
	}

	addr := ":" + cfg.Port
	fmt.Printf("EliteCoach backend listening on %s\n", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}

// newPlanner sceglie il fornitore che genera i piani. Il fallito si prende
// all'avvio e non alla prima generazione: un PLAN_PROVIDER scritto male, o una
// chiave mancante, altrimenti si scoprirebbe solo dopo che un utente ha
// aspettato qualche minuto.
func newPlanner(cfg *config.Config) (service.PlanGenerator, error) {
	switch cfg.PlanProvider {
	case "anthropic":
		// La chiave può restare vuota: l'SDK legge ANTHROPIC_API_KEY
		// dall'ambiente per conto suo.
		return service.NewAnthropicPlanner(cfg.AnthropicKey), nil
	case "deepseek":
		if cfg.DeepSeekKey == "" {
			return nil, fmt.Errorf("PLAN_PROVIDER=deepseek ma DEEPSEEK_API_KEY non è impostata")
		}
		return service.NewDeepSeekPlanner(cfg.DeepSeekKey), nil
	default:
		return nil, fmt.Errorf("PLAN_PROVIDER=%q non riconosciuto: usa \"anthropic\" o \"deepseek\"", cfg.PlanProvider)
	}
}
