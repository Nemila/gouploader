package uploader

import (
	"context"
	"fmt"
	"gouploader/database"
	"io/fs"
	"path/filepath"
	"regexp"
)

var fileNameRe = regexp.MustCompile(`(?i)^(.+?)-(tv|movie)-...`)

func isValidName(fileName string) bool {
	return fileNameRe.MatchString(fileName)
}

func ScanFolder(ctx context.Context, orm *database.Orm, path string) error {
	fmt.Println("scanning media folder")

	files, err := getDirFiles(path)
	if err != nil {
		return err
	}

	for _, filePath := range files {
		if err := orm.Queries.InsertFile(ctx, filePath); err != nil {
			return err
		}
	}

	return nil
}

func getDirFiles(path string) ([]string, error) {
	files := []string{}

	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		fileName := filepath.Base(p)
		if isValid := isValidName(fileName); !isValid {
			return nil
		}

		files = append(files, p)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}
