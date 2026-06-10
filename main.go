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
	Msg string `json:"msg"`
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
	files, err := orm.GetPendingFiles(1, 999999999999999999)
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
		actualUploadAttempted := false

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

				existsOnWebsite, _ := checkFileExists(file.FilePath, hostName)
				if existsOnWebsite {
					_ = orm.AddUpload(file.ID, UploadDone, hostName, "EXISTS", "")
					fmt.Printf("  ⏩ [%s] Already exist on website. Skipping.\n", hostName)
					continue
				}

				updateUploadAttempt(hostName, &actualUploadAttempted)
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

			existsOnWebsite, _ := checkFileExists(file.FilePath, hostName)
			if existsOnWebsite {
				_ = orm.CompleteUpload(file.ID, "EXISTS")
				fmt.Printf("  ⏩ [%s] Already exist on website. Skipping.\n", hostName)
				continue
			}

			if uploadExists.Status == "DONE" {
				fmt.Printf("  ⏩ [%s] Already uploaded previously. Skipping.\n", hostName)
				continue
			}

			if uploadExists.Status == "FAILED" {
				fmt.Printf("  🔄 [%s] Found previous failure. Retrying upload...\n", hostName)
				updateUploadAttempt(hostName, &actualUploadAttempted)
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

		if actualUploadAttempted {
			fmt.Printf("\n⏰  Waiting for 2 minutes before next upload.\n")
			time.Sleep(2 * time.Minute)
		}
	}

	return nil
}

func updateUploadAttempt(hostName string, attempted *bool) {
	ignoreWaitForAdapters := [...]string{"abyss"}
	*attempted = true

	for _, name := range ignoreWaitForAdapters {
		if name == hostName {
			*attempted = false
		}
	}
}

type checkFileExistsResponse struct {
	Exists bool   `json:"exists"`
	Msg    string `json:"msg"`
}

func checkFileExists(filePath, hostName string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	fileName := filepath.Base(file.Name())

	params := url.Values{}
	params.Add("fileName", fileName)
	params.Add("hostName", hostName)

	parsedUrl, err := url.Parse("https://dessinanime.cc/api/import/check-host")
	if err != nil {
		return false, err
	}

	parsedUrl.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, parsedUrl.String(), nil)
	if err != nil {
		return false, err
	}

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}
	res, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	var data checkFileExistsResponse
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return false, err
	}

	if res.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to check %s", data.Msg)
	}

	return data.Exists, nil
}

func importToWebsite(filePath, hostName, slug string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	fileName := filepath.Base(file.Name())

	parsedUrl, err := url.Parse("https://dessinanime.cc/api/import")
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Add("fileName", fileName)
	params.Add("hostName", hostName)
	params.Add("slug", slug)
	parsedUrl.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, parsedUrl.String(), nil)
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout: 2 * time.Minute,
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var importRes importResponse
	if err := json.NewDecoder(res.Body).Decode(&importRes); err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to import to website: %s", importRes.Msg)
	}

	fmt.Printf("  🌐 [API-Import] -> %s\n", importRes.Msg)
	return nil
}
