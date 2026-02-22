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
	telegramProvider "github.com/gsarma/gobot/internal/provider/telegram"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	tg, err := telegramProvider.New(cfg.TelegramToken, cfg.AllowedChatIDs)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}

	exec := claudeexec.New(cfg)

	orch := orchestrator.New(tg, exec, cfg.WorkDir)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("gobot starting...")
	if err := orch.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("orchestrator: %v", err)
	}
	log.Println("gobot stopped")
}
