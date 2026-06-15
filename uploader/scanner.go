package uploader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

// const FILENAME_REGEX =
//   ;

func isValidName(fileName string) bool {
	re := regexp.MustCompile(`(?i)^(.+?)-(tv|movie)-(\d+)-S(\d+)-E(\d+)(?:-(vf|vo|vostfr|multi))?(?:\.([^.]+))?$`)
	return re.MatchString(fileName)
}

func GetDirFiles(path string) ([]string, error) {
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
		return nil, fmt.Errorf("[getDirFiles] walk dir failed: %w", err)
	}

	return files, nil
}
