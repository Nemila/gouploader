package uploader

import (
	"context"
	"fmt"
	"gouploader/database"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

func isValidName(fileName string) bool {
	re := regexp.MustCompile(`(?i)^(.+?)-(tv|movie)-(\d+)-S(\d+)-E(\d+)(?:-(vf|vo|vostfr|multi))?(?:\.([^.]+))?$`)
	return re.MatchString(fileName)
}

func ScanFolder(ctx context.Context, orm *database.Orm, path string) error {
	fmt.Println("scanning media folder")

	files, err := getDirFiles(path)
	if err != nil {
		return err
	}

	for _, filePath := range files {
		if err := orm.Queries.InsertFile(ctx, filePath); err != nil {
			panic(err.Error())
		}
	}

	return nil
}

func getDirFiles(path string) ([]string, error) {
	files := []string{}

	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		file, err := os.Open(p)
		if err != nil {
			return nil
		}

		fileName := filepath.Base(file.Name())
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
