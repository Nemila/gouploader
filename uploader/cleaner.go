package uploader

import (
	"context"
	"fmt"
	"gouploader/config"
	"gouploader/database"
	"gouploader/sqlc"
	"os"
)

func CleanUp(cfg *config.Config, ctx context.Context, orm *database.Orm) error {
	fmt.Println("cleaning up files")

	files, err := orm.Queries.GetFilesByStatus(ctx, "done")
	if err != nil {
		return err
	}

	for _, file := range files {
		uploaded, err := UploadToChannel(cfg.TgToken, cfg.TgEndpoint, file.FilePath)
		if err != nil {
			fmt.Printf("\nfailed to upload to channel: %s", err.Error())
			return err
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
