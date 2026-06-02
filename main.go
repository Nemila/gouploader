package main

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"gouploader/adapters"
	"gouploader/sqlc"

	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

func folderExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		return false
	}

	return info.IsDir()
}

func main() {
	err := godotenv.Load()
	if err != nil {
		panic(err.Error())
	}

	orm, err := NewOrm()
	if err != nil {
		panic(err.Error())
	}
	if err := orm.InitDatabase(); err != nil {
		panic(err.Error())
	}

	mediaPath := os.Getenv("MEDIA_PATH")
	if !folderExists(mediaPath) {
		fmt.Printf("❌ The folder %s does not exist (or is a file).\n", mediaPath)
		return
	}

	for {
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

		if err := processFiles(orm); err != nil {
			panic(err.Error())
		}
		time.Sleep(time.Minute * 5)
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
		_ = orm.UpdateFileStatus(file.ID, FileProcessing)

		uploads, err := orm.GetFileUploads(file.ID)
		if err != nil {
			continue
		}

		allHostsSuccessful := true

		for hostName, adapter := range adapters.Adpaters {
			var uploadExists *sqlc.UploadJob
			for i := range uploads {
				if uploads[i].HostName == hostName {
					uploadExists = &uploads[i]
					break
				}
			}

			if uploadExists == nil {
				slugId, err := adapter.Upload(file.FilePath)
				if err != nil {
					allHostsSuccessful = false
					_ = orm.AddUpload(file.ID, UploadFailed, hostName, "", err.Error())
					fmt.Printf("\nupload failed for host %s(%s): %s", hostName, file.FilePath, err.Error())
					continue
				}
				fmt.Printf("\nupload complete for host %s(%s): %s", hostName, file.FilePath, slugId)
				_ = orm.AddUpload(file.ID, UploadDone, hostName, slugId, "")
				continue
			}

			if uploadExists.Status == "DONE" {
				fmt.Printf("\nskipping file %s for host: %s", file.FilePath, hostName)
				continue
			}

			if uploadExists.Status == "FAILED" {
				slugId, err := adapter.Upload(file.FilePath)
				if err != nil {
					allHostsSuccessful = false
					_ = orm.FailUpload(file.ID, err.Error())
					fmt.Printf("\nupload failed for host %s(%s): %s", hostName, file.FilePath, err.Error())
					continue
				}
				fmt.Printf("\nupload complete for host %s(%s): %s", hostName, file.FilePath, slugId)
				_ = orm.CompleteUpload(file.ID, slugId)
				continue
			}
		}

		if allHostsSuccessful {
			_ = orm.UpdateFileStatus(file.ID, FileDone)
			fmt.Printf("✅ Successfully processed file ID %d across all hosts\n", file.ID)
		} else {
			_ = orm.UpdateFileStatus(file.ID, FilePending)
		}
	}

	return nil
}
