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
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	var p provider.Provider
	var workDirFn func(provider.InboundMessage) string

	switch cfg.Provider {
	case "telegram":
		tg, err := telegramProvider.New(cfg.TelegramToken, cfg.AllowedChatIDs)
		if err != nil {
			log.Fatalf("telegram: %v", err)
		}
		p = tg
		workDirFn = func(_ provider.InboundMessage) string { return cfg.WorkDir }

	case "linear":
		p = linearProvider.New(cfg)
		workDirFn = func(msg provider.InboundMessage) string {
			if msg.Meta != nil {
				if teamKey := msg.Meta["teamKey"]; teamKey != "" {
					if dir, ok := cfg.LinearTeamRepos[teamKey]; ok && dir != "" {
						return dir
					}
				}
			}
			if cfg.LinearDefaultRepo != "" {
				return cfg.LinearDefaultRepo
			}
			return cfg.WorkDir
		}
	}

	exec := claudeexec.New(cfg)
	orch := orchestrator.New(p, exec, workDirFn)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("gobot starting (provider=%s)...", cfg.Provider)
	if err := orch.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("orchestrator: %v", err)
	}
	log.Println("gobot stopped")
}
