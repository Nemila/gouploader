package main

import (
	"context"
	_ "embed"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gouploader/config"
	"gouploader/database"
	"gouploader/uploader"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "modernc.org/sqlite"
)

func main() {
	log := config.NewLogger()

	cfg, err := config.Load()
	if err != nil {
		panic(err.Error())
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	orm, err := database.NewOrm(ctx)
	if err != nil {
		log.Error("Failed to setup database", "err", err)
		return
	}

	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint(cfg.TgToken, cfg.TgEndpoint)
	if err != nil {
		log.Error("Failed to setup telegram bot", "err", err)
		return
	}

	go func() {
		<-ctx.Done()
		log.Warn("Shut down, cleaning up database state")
		if err := orm.Queries.ResetProcessingStatuses(context.Background()); err != nil {
			log.Error("Failed to clean up database", "err", err)
		} else {
			log.Info("Database cleaned up")
		}
	}()

	for {
		if err := uploader.ScanFolder(ctx, orm, cfg.MediaPath); err != nil {
			log.Error("Folder scan failed", "err", err)
		}

		if err := uploader.ProcessFiles(cfg, ctx, orm); err != nil {
			log.Error("Failed to process files", "err", err)
		}

		if err := uploader.CleanUp(bot, cfg, ctx, orm); err != nil {
			log.Error("Failed to clean up", "err", err)
		}

		log.Info("Going to sleep for 5 minutes", "time", time.Now().Add(time.Minute*5).Format("15:04:05"))

		select {
		case <-time.After(time.Minute * 5):
		case <-ctx.Done():
			return
		}
	}
}
