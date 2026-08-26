package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/codersgyan/olx-api/internal/config"
	"github.com/codersgyan/olx-api/internal/db"
	"github.com/codersgyan/olx-api/internal/handlers"
	"github.com/codersgyan/olx-api/internal/middleware"
	"github.com/codersgyan/olx-api/internal/storage"
	"github.com/codersgyan/olx-api/internal/worker"
)

func main() {
	cfg := config.MustLoad()
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
	go wrk.Run(context.TODO())

	fmt.Println(("starting olx server..."))

	lh := handlers.NewListingHandler(db, logger, store)
	ah := handlers.NewAuthHandler(db, logger, cfg)
	uh := handlers.NewUploadHandler(logger, store)

	requireAuth := middleware.RequireAuth(logger, cfg.JwtKey)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handlers.Health)
	mux.HandleFunc("GET /listings", lh.List)
	mux.Handle("DELETE /listings/{id}", requireAuth(http.HandlerFunc(lh.Delete)))
	mux.Handle("POST /listings", requireAuth(http.HandlerFunc(lh.Create)))
	mux.HandleFunc("POST /signup", ah.Signup)
	mux.HandleFunc("POST /signin", ah.Signin)
	mux.Handle("POST /uploads/presign", requireAuth(http.HandlerFunc(uh.Presign)))

	handler := middleware.RequestId(mux)

	srv := http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 30,
		IdleTimeout:  time.Second * 60,
	}

	log.Printf("server is listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
