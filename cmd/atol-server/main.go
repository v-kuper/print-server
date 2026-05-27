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
	"atol-server/internal/printer"
	schedulerruntime "atol-server/internal/scheduler"
	"atol-server/internal/settings"
	"atol-server/internal/web"
)

func main() {
	addr := env("HTTP_ADDR", ":8080")
	settingsPath := env("SETTINGS_PATH", "/data/settings.json")
	libraryPath := env("ATOL_LIBRARY_PATH", "/opt/atol/lib")
	assetsPath := env("ASSETS_PATH", "/opt/atol-server/assets")
	imageEditorPath := env("IMAGE_EDITOR_PATH", "/data/image-editor")

	store := settings.NewStore(settingsPath)
	gateway := printer.NewGateway(libraryPath, assetsPath)
	googleClient := googleintegration.NewClient(googleintegration.DefaultConfig(filepath.Dir(settingsPath)))
	receiptService := app.NewReceiptService(
		store,
		gateway,
		time.Now,
		app.WithGeneratedAssetsPath(assetsPath),
		app.WithGoogleProvider(googleClient),
	)
	scheduler := schedulerruntime.NewService(store, receiptService, time.Now)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go scheduler.Start(ctx)

	server := web.NewServer(
		store,
		gateway,
		time.Now,
		web.WithAssetsPath(assetsPath),
		web.WithImageEditorPath(imageEditorPath),
		web.WithReceiptService(receiptService),
		web.WithGoogleClient(googleClient),
		web.WithScheduler(scheduler),
	)

	log.Printf("ATOL Go Server listening on %s", addr)
	log.Printf("settings path: %s", settingsPath)
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
