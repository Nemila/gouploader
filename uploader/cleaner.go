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
	"path/filepath"

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
		log := log.With("fileName", filepath.Base(file.FilePath))

		if !file.Archived {
			log.Info("Skipping file, not archived yet")
			continue
		}

		if err := os.Remove(file.FilePath); err != nil {
			log.Error("Failed to delete file", "err", err)
		}

		if err := orm.Queries.UpsertFile(ctx, sqlc.UpsertFileParams{
			FilePath: file.FilePath,
			Status:   "saved",
			Archived: true,
		}); err != nil {
			log.Error("Failed to upsert file", "err", err)
		}
	}
	return nil
}

func ArchiveFiles(log *slog.Logger, bot *tgbotapi.BotAPI, cfg *config.Config, ctx context.Context, orm *database.Orm) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	unArchivedFiles, err := orm.Queries.GetUnArchivedFiles(ctx)
	if err != nil {
		return err
	}
	log.Info("Archiving files", "found", len(unArchivedFiles))

	for _, unArchivedFile := range unArchivedFiles {
		log := log.With("fileName", filepath.Base(unArchivedFile.FilePath))

		uploaded, err := UploadToChannel(log, bot, unArchivedFile.FilePath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				log.Info("File doesn't exists (probably already sent)")
				if err := orm.Queries.UpsertFile(ctx, sqlc.UpsertFileParams{
					FilePath: unArchivedFile.FilePath,
					Status:   "saved",
					Archived: true,
				}); err != nil {
					log.Error("Failed to upsert file", "err", err)
				}
			} else {
				log.Error("Failed to upload to telegram", "err", err)
			}
			continue
		}

		if uploaded {
			if err := orm.Queries.UpsertFile(ctx, sqlc.UpsertFileParams{
				FilePath: unArchivedFile.FilePath,
				Status:   unArchivedFile.Status,
				Archived: true,
			}); err != nil {
				log.Error("Failed to upsert file", "err", err)
			}
		}
	}

	return nil
}
