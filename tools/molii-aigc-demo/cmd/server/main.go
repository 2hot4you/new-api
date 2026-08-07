package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"molii-aigc-demo/internal/app"
	"molii-aigc-demo/internal/secure"
	"molii-aigc-demo/internal/store"
	"molii-aigc-demo/internal/upstream"
)

func main() {
	_ = godotenv.Load()

	address := flag.String("addr", envOr("MOLII_DEMO_ADDR", "127.0.0.1:8787"), "HTTP listen address")
	databasePath := flag.String("db", envOr("MOLII_DEMO_DB", "./var/molii-aigc-demo.db"), "SQLite database path")
	flag.Parse()

	masterKey := strings.TrimSpace(os.Getenv("MOLII_DEMO_MASTER_KEY"))
	keyring, err := secure.NewKeyring(masterKey, 1)
	if err != nil {
		fatal("invalid MOLII_DEMO_MASTER_KEY", err)
	}
	decodedKey, err := decodeMasterKey(masterKey)
	if err != nil {
		fatal("derive session secret", err)
	}
	sessionSecret := sha256.Sum256(append([]byte("molii-demo-session\x00"), decodedKey...))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	database, err := store.Open(ctx, *databasePath, keyring, store.DefaultOptions())
	if err != nil {
		fatal("open SQLite", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			slog.Error("close SQLite", "error", closeErr)
		}
	}()

	requestTimeout := durationEnv("MOLII_DEMO_REQUEST_TIMEOUT", 60*time.Second)
	pollInterval := durationEnv("MOLII_DEMO_POLL_INTERVAL", 4*time.Second)
	pollTimeout := durationEnv("MOLII_DEMO_POLL_TIMEOUT", 30*time.Minute)
	pollMaxAttempts := int(pollTimeout / (30 * time.Second))
	if pollMaxAttempts < 10 {
		pollMaxAttempts = 10
	}
	server, err := app.New(app.Config{
		Store: database, Client: upstream.NewClient(requestTimeout, upstream.DefaultResponseCap, nil),
		SessionSecret: sessionSecret[:], SessionTTL: durationEnv("MOLII_DEMO_SESSION_TTL", 24*time.Hour),
		CookieSecure: boolEnv("MOLII_DEMO_COOKIE_SECURE", false),
		AllowedHosts: splitCSV(os.Getenv("MOLII_DEMO_ALLOWED_HOSTS")),
		PollInterval: pollInterval, PollMaxAttempts: pollMaxAttempts,
		BillingSyncPeriod: durationEnv("MOLII_DEMO_BILLING_SYNC_INTERVAL", 5*time.Second),
		Logger:            slog.Default(),
	})
	if err != nil {
		fatal("create Demo server", err)
	}
	server.Start(ctx)

	httpServer := &http.Server{
		Addr: *address, Handler: server.Handler(),
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: requestTimeout + 15*time.Second, IdleTimeout: 90 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("Molii AIGC API Test Lab started", "url", "http://"+*address, "database", *databasePath)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case serveErr := <-errCh:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			fatal("serve HTTP", serveErr)
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP shutdown failed", "error", err)
	}
}

func decodeMasterKey(value string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(value)
	}
	if err != nil || len(decoded) != secure.MasterKeySize {
		return nil, secure.ErrInvalidMasterKey
	}
	return decoded, nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		fatal("invalid "+name, fmt.Errorf("must be a positive Go duration"))
	}
	return parsed
}

func boolEnv(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		fatal("invalid "+name, err)
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	output := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			output = append(output, part)
		}
	}
	return output
}

func fatal(message string, err error) {
	slog.Error(message, "error", err)
	os.Exit(1)
}
