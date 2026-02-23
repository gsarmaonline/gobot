package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gsarma/gobot/internal/config"
	claudeexec "github.com/gsarma/gobot/internal/executor/claude"
	"github.com/gsarma/gobot/internal/orchestrator"
	"github.com/gsarma/gobot/internal/provider"
	linearProvider "github.com/gsarma/gobot/internal/provider/linear"
	telegramProvider "github.com/gsarma/gobot/internal/provider/telegram"
	"github.com/gsarma/gobot/internal/registry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	reg, err := registry.Load(cfg.ProjectsFile)
	if err != nil {
		log.Fatalf("registry: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go reg.Watch(ctx)

	// Build providers from registry data.
	data := reg.Get()
	var providers []provider.Provider
	var tg *telegramProvider.Telegram

	if data.Telegram != nil {
		var err error
		tg, err = telegramProvider.New(data.Telegram.Token, reg)
		if err != nil {
			log.Fatalf("telegram: %v", err)
		}
		providers = append(providers, tg)
		log.Printf("Telegram provider enabled")
	}

	if data.Linear != nil {
		providers = append(providers, linearProvider.New(reg))
		log.Printf("Linear provider enabled")
	}

	if len(providers) == 0 {
		log.Fatalf("no providers configured in %s: add a 'telegram' or 'linear' section", cfg.ProjectsFile)
	}

	workDirFn := func(msg provider.InboundMessage) string {
		if project := msg.Meta["project"]; project != "" {
			if dir := reg.WorkDir(project); dir != "" {
				return dir
			}
		}
		return ""
	}

	opts := orchestrator.Options{
		SessionsFile:    cfg.SessionsFile,
		CICheckInterval: cfg.CICheckInterval,
		CIStuckTimeout:  cfg.CIStuckTimeout,
		MaxCIRetries:    cfg.CIMaxRetries,
	}
	if tg != nil {
		opts.Broadcaster = tg.Broadcast
	}

	exec := claudeexec.New(cfg)
	orch := orchestrator.New(providers, exec, workDirFn, opts)

	log.Printf("gobot starting...")
	if tg != nil {
		tg.Broadcast("gobot started")
	}
	if err := orch.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("orchestrator: %v", err)
	}
	if tg != nil {
		tg.Broadcast("gobot stopping")
	}
	log.Println("gobot stopped")
}
