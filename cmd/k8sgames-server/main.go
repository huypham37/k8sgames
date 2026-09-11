package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/huypham37/k8sgames/internal/cluster"
	"github.com/huypham37/k8sgames/internal/game"
	"github.com/huypham37/k8sgames/internal/server"
)

func main() {
	listen := flag.String("listen", ":8080", "listen address")
	kubectl := flag.String("kubectl", "kubectl", "kubectl executable")
	toolboxImage := flag.String("toolbox-image", "alpine/k8s:1.35.0", "player toolbox image")
	ttl := flag.Duration("session-ttl", 30*time.Minute, "session lifetime")
	maxSessions := flag.Int("max-sessions", 100, "maximum active sessions")
	flag.Parse()

	provider := cluster.NewKubectl(*kubectl, 20*time.Second, *toolboxImage)
	manager := game.NewManager(provider, game.NewCatalog(), *ttl, *maxSessions)
	startup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	err := manager.Cleanup(startup)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "clean stale sessions:", err)
		os.Exit(1)
	}
	handler := server.New(manager)
	httpServer := &http.Server{
		Addr:              *listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go cleanupLoop(ctx, manager)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()

	slog.Info("K8s Games server listening", "address", *listen)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func cleanupLoop(ctx context.Context, manager *game.Manager) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			manager.DeleteExpired(ctx)
		case <-ctx.Done():
			return
		}
	}
}
