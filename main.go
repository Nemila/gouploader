package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
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
				slug, err := adapter.Upload(file.FilePath)
				if err != nil {
					allHostsSuccessful = false
					_ = orm.AddUpload(file.ID, UploadFailed, hostName, "", err.Error())
					fmt.Printf("\nupload failed for host %s(%s): %s", hostName, file.FilePath, err.Error())
					continue
				}
				fmt.Printf("\nupload complete for host %s(%s): %s", hostName, file.FilePath, slug)
				_ = orm.AddUpload(file.ID, UploadDone, hostName, slug, "")
				_ = importToWebsite(file.FilePath, hostName, slug)
				continue
			}

			if uploadExists.Status == "DONE" {
				fmt.Printf("\nskipping file %s for host: %s", file.FilePath, hostName)
				continue
			}

			if uploadExists.Status == "FAILED" {
				slug, err := adapter.Upload(file.FilePath)
				if err != nil {
					allHostsSuccessful = false
					_ = orm.FailUpload(file.ID, err.Error())
					fmt.Printf("\nupload failed for host %s(%s): %s", hostName, file.FilePath, err.Error())
					continue
				}
				fmt.Printf("\nupload complete for host %s(%s): %s", hostName, file.FilePath, slug)
				_ = orm.CompleteUpload(file.ID, slug)
				_ = importToWebsite(file.FilePath, hostName, slug)
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

func importToWebsite(filePath, hostName, slug string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("[addToWebsite] failed to open file: %w", err)
	}
	defer file.Close()
	fileName := filepath.Base(file.Name())

	baseUrl := "https://dessinanime.cc/api/import"
	ctx := context.Background()
	client := &http.Client{
		Timeout: 2 * time.Minute,
	}

	parsedUrl, err := url.Parse(baseUrl)
	if err != nil {
		return fmt.Errorf("[addToWebsite] failed to parse url: %w", err)
	}

	params := url.Values{}
	params.Add("fileName", fileName)
	params.Add("hostName", hostName)
	params.Add("slug", slug)

	parsedUrl.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedUrl.String(), nil)
	if err != nil {
		return fmt.Errorf("[addToWebsite] failed to create request: %w", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("[addToWebsite] faied to execute request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		var importRes importResponse
		if err := json.NewDecoder(res.Body).Decode(&importRes); err != nil {
			return fmt.Errorf("[addToWebsite] faied to decode json: %w", err)
		}
		fmt.Println(importRes.Message)
	}

	return nil
}
