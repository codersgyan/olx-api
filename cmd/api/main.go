package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codersgyan/olx-api/internal/config"
	"github.com/codersgyan/olx-api/internal/db"
	"github.com/codersgyan/olx-api/internal/handlers"
	"github.com/codersgyan/olx-api/internal/middleware"
	"github.com/codersgyan/olx-api/internal/storage"
	"github.com/codersgyan/olx-api/internal/worker"
	"golang.org/x/time/rate"
)

const (
	serverShutdownTimeout = 15 * time.Second
	workerShutdownTimeout = 10 * time.Second
)

func main() {
	cfg := config.MustLoad()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := db.Connect(cfg.DatabaseUrl)
	if err != nil {
		log.Fatalf("main.db.connect: %v", err)
	}

	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	})
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	fmt.Println(("database connected"))

	// storage initialisation
	store, err := storage.NewR2(context.TODO(), storage.R2Config{
		AccountID:    cfg.StorageAccountID,
		AccessKey:    cfg.StorageAccessKey,
		AccessSecret: cfg.StorageAccessSecret,
		Bucket:       cfg.StorageBucket,
	})
	if err != nil {
		log.Fatalf("main.storage.r2: %v", err)
	}

	fmt.Println(("storage initialised..."))

	// start worker
	wrk := worker.New(db, store, logger)
	go wrk.Run(ctx)

	fmt.Println(("starting olx server..."))

	lh := handlers.NewListingHandler(db, logger, store)
	ah := handlers.NewAuthHandler(db, logger, cfg)
	uh := handlers.NewUploadHandler(logger, store)

	globalLimiter := middleware.RateLimit(logger, rate.Every(time.Second), 100)
	// 60 secs/requests per minute = 60 / 500 = 0.12 secs
	signinLimiter := middleware.RateLimit(logger, rate.Every(time.Second*5), 5) // 1 token every 12 second - 5r/min
	signupLimiter := middleware.RateLimit(logger, rate.Every(time.Minute), 3)
	requireAuth := middleware.RequireAuth(logger, cfg.JwtKey)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handlers.Health)
	mux.HandleFunc("GET /listings", lh.List)
	mux.Handle("DELETE /listings/{id}", requireAuth(http.HandlerFunc(lh.Delete)))
	mux.Handle("POST /listings", requireAuth(http.HandlerFunc(lh.Create)))
	mux.Handle("POST /signup", signupLimiter(http.HandlerFunc(ah.Signup)))
	mux.Handle("POST /signin", signinLimiter(http.HandlerFunc(ah.Signin)))
	mux.Handle("POST /uploads/presign", requireAuth(http.HandlerFunc(uh.Presign)))

	handler := globalLimiter(middleware.RequestId(mux))

	srv := http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 30,
		IdleTimeout:  time.Second * 60,
	}

	go func() {
		log.Printf("server is listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	<-ctx.Done()
	// escape hatch double ctrl+c
	stop()

	logger.Info("shutting down...")
	srvShutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(srvShutdownCtx); err != nil {
		logger.Error("server shutdown failed", "err", err)
	}

	select {
	case <-wrk.Done():
	case <-time.After(workerShutdownTimeout):
		logger.Warn("worker still busy, exiting anyway.")
	}

	db.Close()
	logger.Info("bye")
}
