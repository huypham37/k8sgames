package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/huypham37/k8sgames/internal/game"
	"github.com/huypham37/k8sgames/internal/progress"
	"github.com/huypham37/k8sgames/internal/terminal"
)

func main() {
	server := flag.String("server", env("K8SGAMES_SERVER", "http://localhost:8080"), "K8s Games server URL")
	challenge := flag.String("challenge", "", "play one challenge by ID")
	list := flag.Bool("list", false, "list available challenges")
	flag.Parse()

	client, err := terminal.NewClient(*server, &http.Client{Timeout: 2 * time.Minute})
	if err != nil {
		fail(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	challenges, err := client.Challenges(ctx)
	if err != nil {
		fail(err)
	}
	progressPath, err := progress.DefaultPath()
	if err != nil {
		fail(err)
	}
	store, err := progress.Open(env("K8SGAMES_PROGRESS", progressPath))
	if err != nil {
		fail(err)
	}
	if *list {
		for _, item := range challenges {
			mark := " "
			if store.Has(item.ID) {
				mark = "✓"
			}
			fmt.Printf("[%s] %-20s %s\n", mark, item.ID, item.Title)
		}
		return
	}
	menu := terminal.NewMenu(os.Stdin, os.Stdout)
	for {
		selected := []string{*challenge}
		if *challenge == "" {
			selected, err = menu.Select(challenges, store.Has)
			if err != nil {
				fail(err)
			}
		}
		if len(selected) == 0 {
			return
		}
		for index, id := range selected {
			if index > 0 {
				fmt.Println("\nStarting next challenge...")
			}
			completed, err := client.Play(ctx, id, os.Stdin, os.Stdout)
			if err != nil && ctx.Err() == nil {
				fail(err)
			}
			if !completed {
				return
			}
			if err := store.Complete(id); err != nil {
				fail(err)
			}
		}
		if allComplete(challenges, store.Has) {
			fmt.Println("\nAll challenges complete!")
			return
		}
		if *challenge != "" {
			return
		}
	}
}

func allComplete(challenges []game.Challenge, completed func(string) bool) bool {
	for _, challenge := range challenges {
		if !completed(challenge.ID) {
			return false
		}
	}
	return len(challenges) > 0
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "k8sgames:", err)
	os.Exit(1)
}
