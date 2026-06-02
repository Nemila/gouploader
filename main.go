package main

import (
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gouploader/adapters"
	"gouploader/sqlc"

	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

func displayMenu() {
	fmt.Println("\nWHAT WOULD YOU LIKE TO DO ?")
	fmt.Println("0 - EXIT")
	fmt.Println("1 - REGISTER FILES")
	fmt.Println("2 - DISPLAY PENDING FILES")
	fmt.Println("3 - UPLOAD FILE TO ABYSS")
	fmt.Println("4 - PROCESS FILES")
}

func main() {
	err := godotenv.Load()
	if err != nil {
		panic(err.Error())
	}

	var choice int = 1

	for choice != 0 {
		displayMenu()
		fmt.Scanln(&choice)

		orm, err := NewOrm()
		if err != nil {
			panic(err.Error())
		}

		if err := orm.InitDatabase(); err != nil {
			panic(err.Error())
		}

		switch choice {
		case 1:
			files, err := getDirFiles(os.Getenv("MEDIA_PATH"))
			if err != nil {
				panic(err.Error())
			}
			for _, file := range files {
				err := orm.RegisterFile(file)
				if err != nil {
					panic(err.Error())
				}
			}
		case 2:
			pendingFiles, err := orm.GetPendingFiles(1, 20)
			if err != nil {
				panic(err.Error())
			}
			fmt.Println(pendingFiles)
		case 3:
			if _, err := adapters.Adpaters["vidhide"].Upload("/home/nemila/Videos/go_http_tutorial.mp4"); err != nil {
				panic(err.Error())
			}
		case 4:
			if err := processFiles(orm); err != nil {
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
		return nil, fmt.Errorf("[getDirFiles] walk dir failed: %w", err)
	}

	return files, nil
}

func processFiles(orm *Orm) error {
	files, err := orm.GetPendingFiles(1, 20)
	if err != nil {
		return fmt.Errorf("[processFiles] failed to get pending files: %w", err)
	}

	for _, file := range files {
		_ = orm.UpdateFileStatus(file.ID, FileProcessing, "")

		uploads, err := orm.GetFileUploads(file.ID)
		if err != nil {
			orm.UpdateFileStatus(file.ID, FilePending, err.Error())
			continue
		}

		for hostName, adapter := range adapters.Adpaters {
			var uploadExists *sqlc.UploadJob
			for i := range uploads {
				if uploads[i].HostName == hostName {
					uploadExists = &uploads[i] // 100% safe reference directly into the slice
					break
				}
			}

			if uploadExists == nil {
				slugId, err := adapter.Upload(file.FilePath)
				if err != nil {
					_ = orm.UpdateFileStatus(file.ID, FilePending, "")
					_ = orm.AddUpload(file.ID, UploadFailed, hostName, "", err.Error())
					continue
				}
				_ = orm.AddUpload(file.ID, UploadDone, hostName, slugId, "")
				continue
			}

			if uploadExists.Status == "DONE" {
				continue
			}

			if uploadExists.Status == "FAILED" {
				slugId, err := adapter.Upload(file.FilePath)
				if err != nil {
					_ = orm.UpdateFileStatus(file.ID, FilePending, "")
					_ = orm.FailUpload(file.ID, err.Error())
					continue
				}
				_ = orm.CompleteUpload(file.ID, slugId)
				continue
			}
		}

	}

	return nil
}
