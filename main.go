package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gouploader/config"
	"gouploader/database"
	"gouploader/uploader"

	_ "modernc.org/sqlite"
)

type importResponse struct {
	Msg string `json:"msg"`
}

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
		panic(err.Error())
	}

	go func() {
		<-ctx.Done()
		log.Warn("shutdown signal received, cleaning up database state")
		if err := orm.Queries.ResetProcessingStatuses(context.Background()); err != nil {
			log.Error("failed to clean up database state", "err", err)
		} else {
			log.Info("database successfully cleaned up")
		}
		os.Exit(0)
	}()

	for {

		if err := uploader.ScanFolder(ctx, orm, cfg.MediaPath); err != nil {
			panic(err.Error())
		}

		if err := uploader.ProcessFiles(cfg, ctx, orm); err != nil {
			panic(err.Error())
		}

		if err := uploader.CleanUp(cfg, ctx, orm); err != nil {
			panic(err.Error())
		}

		fmt.Printf("\nGoing to sleep for 5 minutes: %s", time.Now().Add(time.Minute*5).Format("15:04:05"))
		time.Sleep(time.Minute * 5)
	}
}
