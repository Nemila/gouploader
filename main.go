package main

import (
	_ "embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"

	"gouploader/adapters"
	"gouploader/sqlc"

	// "github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

var mediaPath = "/home/nemila/Videos"

func displayMenu() {
	fmt.Println("\nWHAT WOULD YOU LIKE TO DO ?")
	fmt.Println("0 - EXIT")
	fmt.Println("1 - PUSH CHANGES TO DATABASE")
	fmt.Println("2 - REGISTER FILES")
	fmt.Println("3 - DISPLAY PENDING FILES")
	fmt.Println("4 - UPLOAD FILE TO ABYSS")
}

func main() {
	// if err != nil {
	// 	panic(err.Error())
	// }

	var choice int = 1

	for choice != 0 {
		displayMenu()
		fmt.Scanln(&choice)

		orm, err := NewOrm()
		if err != nil {
			panic(err.Error())
		}

		switch choice {
		case 1:
			orm.Migrate()
		case 2:
			files, err := getDirFiles(mediaPath)
			if err != nil {
				panic(err.Error())
			}
			for _, file := range files {
				err := orm.RegisterFile(file)
				if err != nil {
					panic(err.Error())
				}
			}
		case 3:
			pendingFiles, err := orm.GetPendingFiles(1, 20)
			if err != nil {
				panic(err.Error())
			}
			fmt.Println(pendingFiles)
		case 4:
			_, err := adapters.Adpaters["uqload"].Upload("/home/nemila/Videos/video.mp4")
			if err != nil {
				panic(err.Error())
			}
		}
	}

}

func getDirFiles(path string) ([]string, error) {
	files := []string{}

	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return files, err
	}

	return files, nil
}

func processFiles(orm *Orm) error {
	files, err := orm.GetPendingFiles(1, 20)
	if err != nil {
		return err
	}

	for _, file := range files {
		err := orm.UpdateFileStatus(PROCESSING, "", file.ID)
		if err != nil {
			orm.UpdateFileStatus(PENDING, err.Error(), file.ID)
			continue
		}

		uploads, err := orm.GetFileUploads(file.ID)
		if err != nil {
			orm.UpdateFileStatus(PENDING, err.Error(), file.ID)
			continue
		}

		for hostName, adapter := range adapters.Adpaters {
			uploadExists := slices.ContainsFunc(uploads, func(u sqlc.UploadJob) bool {
				if u.HostName == hostName {
					return true
				}
				return false
			})

			if !uploadExists {
				// TODO: UPLOAD
				_, err := adapter.Upload(file.FilePath)
				if err != nil {
					// TODO: UPDATE UPLOAD STATUS AND ADD ERROR MESSAGE
					continue
				}
				// TODO: CREATE UPLOAD

			}

			// IF SUCCESS SKIP
			// IF FAILED RETRY

		}
	}
	return nil
}
