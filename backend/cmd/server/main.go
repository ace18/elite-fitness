package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

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

	// services
	authSvc := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.IsDev())
	workoutSvc := service.NewWorkoutService(programRepo)
	aiSvc := service.NewAIService(cfg.AnthropicKey, programRepo, pool)

	// handlers
	authH := handler.NewAuthHandler(authSvc, userRepo)
	programH := handler.NewProgramHandler(programRepo, aiSvc)
	workoutH := handler.NewWorkoutHandler(workoutSvc)
	sessionH := handler.NewSessionHandler(sessionRepo)
	progressH := handler.NewProgressHandler(sessionRepo, programRepo)

	// router
	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// public auth routes
	r.Post("/api/auth/magic-link", authH.SendMagicLink)
	r.Get("/api/auth/verify", authH.Verify)

	// protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(authSvc))

		r.Get("/api/auth/me", authH.Me)
		r.Get("/api/workout/today", workoutH.GetToday)
		r.Get("/api/program", programH.GetProgram)
		r.Post("/api/program", programH.SetProgram)
		r.Get("/api/plans", programH.GetPlans)
		r.Post("/api/plans/generate", programH.GeneratePlan)
		r.Post("/api/sessions", sessionH.SaveSession)
		r.Get("/api/sessions/last", sessionH.GetLastSession)
		r.Get("/api/progress", progressH.GetProgress)
		r.Post("/api/progress/weight", progressH.LogWeight)
	})

	addr := ":" + cfg.Port
	fmt.Printf("EliteCoach backend listening on %s\n", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
	_ = os.Getenv // suppress unused import
}
