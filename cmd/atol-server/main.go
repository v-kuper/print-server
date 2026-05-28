package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"atol-server/internal/app"
	"atol-server/internal/googleintegration"
	"atol-server/internal/imageeditor"
	"atol-server/internal/printer"
	"atol-server/internal/receiptsnapshot"
	schedulerruntime "atol-server/internal/scheduler"
	"atol-server/internal/settings"
	"atol-server/internal/storage"
	"atol-server/internal/web"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := env("HTTP_ADDR", ":8080")
	settingsPath := env("SETTINGS_PATH", "/data/settings.json")
	libraryPath := env("ATOL_LIBRARY_PATH", "/opt/atol/lib")
	assetsPath := env("ASSETS_PATH", "/opt/atol-server/assets")
	imageEditorPath := env("IMAGE_EDITOR_PATH", "/data/image-editor")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if err := storage.Migrate(ctx, databaseURL); err != nil {
		log.Fatalf("postgres migration failed: %v", err)
	}
	pool, err := storage.OpenPool(ctx, databaseURL)
	if err != nil {
		log.Fatalf("postgres connection failed: %v", err)
	}
	defer pool.Close()

	store, err := settings.NewPostgresStore(ctx, pool)
	if err != nil {
		log.Fatalf("postgres settings store failed: %v", err)
	}
	if err := store.ImportLegacySettings(ctx, settingsPath); err != nil {
		log.Fatalf("legacy settings import failed: %v", err)
	}
	imageStore := imageeditor.NewPostgresStore(pool, store.WorkspaceID(), time.Now)
	if err := imageStore.ImportLegacy(ctx, imageEditorPath); err != nil {
		log.Fatalf("legacy image editor import failed: %v", err)
	}
	googleTokenStore := googleintegration.NewPostgresTokenStore(pool, store.WorkspaceID())
	receiptSnapshotStore := receiptsnapshot.NewPostgresStore(pool, store.WorkspaceID())
	googleConfig := googleintegration.DefaultConfig(filepath.Dir(settingsPath))
	if err := googleTokenStore.ImportLegacyToken(ctx, googleConfig.TokenPath); err != nil {
		log.Fatalf("legacy Google token import failed: %v", err)
	}
	googleConfig.TokenStore = googleTokenStore
	gateway := printer.NewGateway(libraryPath, assetsPath)
	googleClient := googleintegration.NewClient(googleConfig)
	receiptService := app.NewReceiptService(
		store,
		gateway,
		time.Now,
		app.WithGeneratedAssetsPath(assetsPath),
		app.WithGoogleProvider(googleClient),
		app.WithReceiptSnapshotStore(receiptSnapshotStore),
	)
	scheduler := schedulerruntime.NewService(store, receiptService, time.Now)
	go scheduler.Start(ctx)

	server := web.NewServer(
		store,
		gateway,
		time.Now,
		web.WithAssetsPath(assetsPath),
		web.WithImageEditorPath(imageEditorPath),
		web.WithImageEditorStore(imageStore),
		web.WithReceiptService(receiptService),
		web.WithReceiptSnapshotStore(receiptSnapshotStore),
		web.WithGoogleClient(googleClient),
		web.WithScheduler(scheduler),
	)

	log.Printf("ATOL Go Server listening on %s", addr)
	log.Printf("settings path: %s", settingsPath)
	log.Printf("postgres storage: enabled")
	log.Printf("ATOL library path: %s", libraryPath)
	log.Printf("assets path: %s", assetsPath)
	log.Printf("image editor path: %s", imageEditorPath)
	googleStatus := googleClient.Status()
	log.Printf("Google credentials path: %s", googleStatus.CredentialsPath)
	log.Printf("Google token path: %s", googleStatus.TokenPath)

	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
