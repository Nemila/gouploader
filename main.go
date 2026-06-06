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
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gouploader/adapters"
	"gouploader/sqlc"

	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

type importResponse struct {
	Message string `json:"message"`
}

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

	// adapter := adapters.Adapters["sendvid"]
	// sendvid := adapter.(*adapters.Sendvid)
	// if _, err := sendvid.Upload("/home/nemila/Videos/video.mp4"); err != nil {
	// 	panic(err.Error())
	// }
	// return

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		fmt.Println("\n🛑 Shutdown signal received! Cleaning up database states...")
		err := orm.ResetProcessingStatuses()
		if err != nil {
			fmt.Printf("❌ Failed to reset statuses during shutdown: %v\n", err)
		} else {
			fmt.Println("✅ Active database states successfully reverted to Pending.")
		}
		os.Exit(0)
	}()

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

		if err := processFiles(ctx, orm); err != nil {
			panic(err.Error())
		}

		fmt.Printf("\n[💤] Going to sleep for 5 minutes... (Next cycle at: %s)\n", time.Now().Add(time.Minute*5).Format("15:04:05"))
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

func processFiles(ctx context.Context, orm *Orm) error {
	files, err := orm.GetPendingFiles(1, 20)
	if err != nil {
		return fmt.Errorf("[processFiles] failed to get pending files: %w", err)
	}

	for _, file := range files {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		_ = orm.UpdateFileStatus(file.ID, FileProcessing)

		uploads, err := orm.GetFileUploads(file.ID)
		if err != nil {
			continue
		}

		allHostsSuccessful := true

		fmt.Println("\n──────────────────────────────────────────────────────────")
		fmt.Printf("📦 PROCESSING FILE ID [%d]\n", file.ID)
		fmt.Printf("📄 Path: %s\n", file.FilePath)
		fmt.Println("──────────────────────────────────────────────────────────")

		for hostName, adapter := range adapters.Adapters {
			if ctx.Err() != nil {
				_ = orm.UpdateFileStatus(file.ID, FilePending)
				return ctx.Err()
			}

			var uploadExists *sqlc.UploadJob
			for i := range uploads {
				if uploads[i].HostName == hostName {
					uploadExists = &uploads[i]
					break
				}
			}

			if uploadExists == nil {
				fmt.Printf("  ⚡ [%s] Initiating fresh upload...\n", hostName)
				slug, err := adapter.Upload(file.FilePath)
				if err != nil {
					allHostsSuccessful = false
					_ = orm.AddUpload(file.ID, UploadFailed, hostName, "", err.Error())
					fmt.Printf("  ❌ [%s] Upload Failed: %s\n", hostName, err.Error())
					continue
				}
				fmt.Printf("  ✨ [%s] Complete! -> Slug: %s\n", hostName, slug)
				_ = orm.AddUpload(file.ID, UploadDone, hostName, slug, "")
				_ = importToWebsite(file.FilePath, hostName, slug)
				continue
			}

			if uploadExists.Status == "DONE" {
				fmt.Printf("  ⏩ [%s] Already uploaded previously. Skipping.\n", hostName)
				continue
			}

			if uploadExists.Status == "FAILED" {
				fmt.Printf("  🔄 [%s] Found previous failure. Retrying upload...\n", hostName)
				slug, err := adapter.Upload(file.FilePath)
				if err != nil {
					allHostsSuccessful = false
					_ = orm.FailUpload(file.ID, err.Error())
					fmt.Printf("  ❌ [%s] Retry Failed: %s\n", hostName, err.Error())
					continue
				}
				fmt.Printf("  ✨ [%s] Retry Complete! -> Slug: %s\n", hostName, slug)
				_ = orm.CompleteUpload(file.ID, slug)
				_ = importToWebsite(file.FilePath, hostName, slug)
				continue
			}
		}

		if allHostsSuccessful {
			_ = orm.UpdateFileStatus(file.ID, FileDone)
			fmt.Printf("✔️  SUCCESS: File ID %d successfully handled across all hosts.\n", file.ID)
		} else {
			_ = orm.UpdateFileStatus(file.ID, FilePending)
			fmt.Printf("⚠️  PARTIAL COMPLETION: Some uploads failed for File ID %d. Marked back to pending.\n", file.ID)
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
		fmt.Printf("  🌐 [API-Import] -> %s\n", importRes.Message)
	}

	return nil
}
