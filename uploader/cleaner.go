package uploader

import (
	"context"
	"errors"
	"gouploader/config"
	"gouploader/database"
	"gouploader/sqlc"
	"io/fs"
	"log/slog"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func CleanUp(log *slog.Logger, bot *tgbotapi.BotAPI, cfg *config.Config, ctx context.Context, orm *database.Orm) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	files, err := orm.Queries.GetFilesByStatus(ctx, "done")
	if err != nil {
		return err
	}
	log.Info("Cleaning up files", "found", len(files))

	for _, file := range files {
		log = log.With("filePath", file.FilePath)

		uploaded, err := UploadToChannel(log, bot, file.FilePath)
		if err != nil {
			log.Error("Failed to upload to telegram", "err", err)
			continue
		}

		if uploaded {
			if err := orm.Queries.UpsertFile(ctx, sqlc.UpsertFileParams{
				FilePath: file.FilePath,
				Status:   "saved",
			}); err != nil {
				log.Error("Failed to upsert file", "err", err)
			}

			if err := os.Remove(file.FilePath); err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					log.Info("File doesn't exists (probably already sent)")
					if err := orm.Queries.UpsertFile(ctx, sqlc.UpsertFileParams{
						FilePath: file.FilePath,
						Status:   "saved",
					}); err != nil {
						log.Error("Failed to upsert file", "err", err)
					}
				} else {
					log.Error("Failed to delete file", "err", err)
				}
			}
		}
	}
	return nil
}
