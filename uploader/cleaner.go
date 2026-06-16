package uploader

import (
	"context"
	"fmt"
	"gouploader/config"
	"gouploader/database"
	"gouploader/sqlc"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func CleanUp(bot *tgbotapi.BotAPI, cfg *config.Config, ctx context.Context, orm *database.Orm) error {
	fmt.Println("Cleaning up files")

	files, err := orm.Queries.GetFilesByStatus(ctx, "done")
	if err != nil {
		return err
	}

	for _, file := range files {
		fmt.Printf("\nUploading file to telegram: %s", file.FilePath)
		uploaded, err := UploadToChannel(bot, file.FilePath)
		if err != nil {
			fmt.Printf("\nFailed to upload to telegram: %v", err)
			continue
		}

		if uploaded {
			orm.Queries.UpsertFile(ctx, sqlc.UpsertFileParams{
				FilePath: file.FilePath,
				Status:   "saved",
			})
			os.Remove(file.FilePath)
		}
	}

	return nil
}
