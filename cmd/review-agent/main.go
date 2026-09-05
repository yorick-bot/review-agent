package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vacp2p/review-agent/internal/config"
	"github.com/vacp2p/review-agent/internal/github"
	"github.com/vacp2p/review-agent/internal/reviewer"
	"github.com/vacp2p/review-agent/internal/webhook"
)

func main() {
	var (
		cfgPath     string
		checkConfig bool
	)
	flag.StringVar(&cfgPath, "config", "/etc/review-agent/config.yaml", "path to config yaml")
	flag.BoolVar(&checkConfig, "check-config", false, "validate config + required env, then exit")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Error("config load", "err", err)
		os.Exit(1)
	}
	secret := os.Getenv("WEBHOOK_SECRET")
	token := os.Getenv("GITHUB_TOKEN")
	if secret == "" || token == "" {
		log.Error("WEBHOOK_SECRET and GITHUB_TOKEN required")
		os.Exit(1)
	}
	if checkConfig {
		log.Info("config ok", "path", cfgPath)
		return
	}

	gh := github.New(token)
	reg := reviewer.NewRegistry()
	reg.Register(reviewer.Copilot{
		Client:        gh,
		ReviewerLogin: copilotLogin(cfg),
		BotUser:       cfg.BotUser,
	})
	for name := range cfg.Reviewers {
		if name == "copilot" {
			continue
		}
		reg.Register(reviewer.AIStub{NameStr: name})
	}

	h := &webhook.Handler{
		Secret:   []byte(secret),
		Config:   cfg,
		Registry: reg,
		Log:      log,
	}
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func copilotLogin(cfg *config.Config) string {
	if r, ok := cfg.Reviewers["copilot"]; ok {
		if v := r["reviewer_login"]; v != "" {
			return v
		}
	}
	return "Copilot"
}
