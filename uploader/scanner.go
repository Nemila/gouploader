package uploader

import (
	"context"
	"gouploader/database"
	"io/fs"
	"log/slog"
	"path/filepath"
	"regexp"
)

var fileNameRe = regexp.MustCompile(
	`(?i)^(?P<name>.+?)-(?P<mediaType>tv|movie)-(?P<tmdbId>\d+)-S(?P<season>\d+)-E(?P<episode>\d+)(?:-(?P<lang>vf|vo|vostfr|multi))?(?:\.(?P<ext>[^.]+))?$`,
)

func ScanFolder(log *slog.Logger, ctx context.Context, orm *database.Orm, paths []string) error {
	for _, path := range paths {
		log.Info("Scanning media folder", "folder", path)

		files := getDirFiles(log, path)
		for _, filePath := range files {
			if err := orm.Queries.InsertFile(ctx, filePath); err != nil {
				log.Error("Failed to insert file", "file", filePath)
				continue
			}
		}
	}
	return nil
}

func getDirFiles(log *slog.Logger, path string) []string {
	filePaths := []string{}

	_ = filepath.WalkDir(path, func(path string, dir fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if dir.IsDir() {
			return nil
		}

		if isValid := fileNameRe.MatchString(filepath.Base(path)); !isValid {
			return nil
		}

		filePaths = append(filePaths, path)
		return nil
	})

	log.Info("Done scanning", "found", len(filePaths))
	return filePaths
}
