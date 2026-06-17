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

var cooldownTime = 5 * time.Minute

func main() {
	log := config.NewLogger()

	cfg, err := config.Load()
	if err != nil {
		log.Error("Failed loading config", "err", err)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	orm, err := database.NewOrm(ctx)
	if err != nil {
		log.Error("Failed setting up database", "err", err)
		return
	}

	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint(cfg.TgToken, cfg.TgEndpoint)
	if err != nil {
		log.Error("Failed setting up telegram bot", "err", err)
		return
	}

	go func() {
		<-ctx.Done()
		log.Warn("Shut down signal, cleaning up database state")
		if err := orm.Queries.ResetProcessingStatuses(context.Background()); err != nil {
			log.Error("Failed to clean up database", "err", err)
		} else {
			log.Info("Database cleaned up")
		}
	}()

	for {
		if err := uploader.ScanFolder(log, ctx, orm, []string{cfg.MediaPath, "/home/nemila/uploaders"}); err != nil {
			log.Error("Folder scan failed", "err", err)
		}

		if err := uploader.ProcessFiles(log, cfg, ctx, orm); err != nil {
			log.Error("Failed to process files", "err", err)
		}

		if err := uploader.ArchiveFiles(log, bot, cfg, ctx, orm); err != nil {
			log.Error("Failed to archive files", "err", err)
		}

		if err := uploader.CleanUp(log, bot, cfg, ctx, orm); err != nil {
			log.Error("Failed to clean up", "err", err)
		}

		log.Info("Sleeping for 5 minutes", "time", time.Now().Add(cooldownTime).Format("15:04:05"))

		select {
		case <-time.After(cooldownTime):
		case <-ctx.Done():
			return
		}
	}
}
