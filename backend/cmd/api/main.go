package main

// @title Friendzone API
// @version 1.0
// @description Friendzone authentication and profile API
// @BasePath /

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BallSpins/friendzone/docs"
	"github.com/BallSpins/friendzone/internal/auth"
	"github.com/BallSpins/friendzone/internal/chat"
	"github.com/BallSpins/friendzone/internal/config"
	"github.com/BallSpins/friendzone/internal/embedding"
	"github.com/BallSpins/friendzone/internal/match"
	"github.com/BallSpins/friendzone/internal/middleware"
	"github.com/BallSpins/friendzone/internal/user"
	"github.com/BallSpins/friendzone/internal/workspace"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

// userProvisionerAdapter menjembatani user.Service ke kontrak auth.UserProvisioner
// tanpa membuat package auth mengimpor package user secara langsung.
type userProvisionerAdapter struct {
	userService *user.Service
}

func (a *userProvisionerAdapter) SyncGoogleAccountID(ctx context.Context, sub, email, name, picture string) (string, error) {
	u, err := a.userService.SyncGoogleAccount(ctx, sub, email, name, picture)
	if err != nil {
		return "", err
	}
	return u.ID, nil
}

func main() {
	cfg := config.Load()

	docs.SwaggerInfo.Host = fmt.Sprintf("localhost:%s", cfg.AppPort)
	if cfg.IsProduction {
		docs.SwaggerInfo.Schemes = []string{"https"}
	} else {
		docs.SwaggerInfo.Schemes = []string{"http"}
	}

	// 1. Inisialisasi PostgreSQL
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to open PostgreSQL connection: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	cancel()

	// 2. Inisialisasi Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	defer rdb.Close()

	ctxRedis, cancelRedis := context.WithTimeout(context.Background(), 5*time.Second)
	if err := rdb.Ping(ctxRedis).Err(); err != nil {
		log.Fatalf("Failed to ping Redis: %v", err)
	}
	cancelRedis()

	// 3. Dependency Injection (Repository -> Service -> Handler)
	embedder, err := embedding.NewGeminiEmbedder(context.Background(), cfg.GeminiApiKey)
	if err != nil {
		log.Fatalf("Failed to initialize Gemini embedder: %v", err)
	}
	userRepo := user.NewPostgresRepository(db)
	userService := user.NewService(userRepo, embedder)

	sessionStore := auth.NewRedisSessionStore(rdb)
	authService := auth.NewService(cfg, &userProvisionerAdapter{userService: userService}, sessionStore)
	authHandler := auth.NewHandler(authService, cfg.IsProduction)
	userHandler := user.NewHandler(userService, authService)

	matchRepo := match.NewPostgresRepository(db)
	matchService := match.NewService(matchRepo, rdb)
	matchHandler := match.NewHandler(matchService, authService)

	wsRepo := workspace.NewPostgresRepository(db)
	wsService := workspace.NewService(wsRepo, rdb)
	wsHandler := workspace.NewHandler(wsService, authService)

	chatService := chat.NewService(db, rdb)
	chatHandler := chat.NewHandler(chatService, authService)

	// 4. Routing
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<a href="/auth/google/login">Sign in with Google</a>`)
	})

	authHandler.RegisterRoutes(mux)
	userHandler.RegisterRoutes(mux)
	matchHandler.RegisterRoutes(mux)
	wsHandler.RegisterRoutes(mux)
	chatHandler.RegisterRoutes(mux)
	
	mux.HandleFunc("GET /docs/", httpSwagger.Handler(
		httpSwagger.URL("/docs/doc.json"),
	))

	handlerWithCORS := middleware.CORS(cfg.AllowedOrigins, mux)

	// 5. HTTP Server & Graceful Shutdown
	server := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      handlerWithCORS,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server listening on http://localhost:%s", cfg.AppPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Forced server shutdown: %v", err)
	}
	log.Println("Server exited properly")
}
