package uploader

import (
	"context"
	"fmt"
	"gouploader/adapters"
	"gouploader/config"
	"gouploader/database"
	"gouploader/sqlc"
	"gouploader/website"
	"net/http"
	"sort"
	"time"
)

const uploadCooldown = 2 * time.Minute

func ProcessFiles(cfg *config.Config, ctx context.Context, orm *database.Orm) error {
	files, err := orm.GetPendingFiles(1, 999999999999999999)
	if err != nil {
		return err
	}

	wc := &website.Client{
		BaseUrl: cfg.WebsiteUrl,
		HttpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}

	for _, file := range files {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := processFile(ctx, orm, wc, file); err != nil {
			return err
		}
	}

	return nil
}

func processFile(ctx context.Context, orm *database.Orm, wc *website.Client, file sqlc.File) error {
	fmt.Println("\n──────────────────────────────────────────────────────────")
	fmt.Printf("📦 PROCESSING FILE ID [%d]\n", file.ID)
	fmt.Printf("📄 Path: %s\n", file.FilePath)
	fmt.Println("──────────────────────────────────────────────────────────")

	if err := orm.UpdateFileStatus(file.ID, database.FileProcessing); err != nil {
		fmt.Printf("  ⚠️  Could not mark file %d as processing: %v\n", file.ID, err)
	}

	uploads, err := orm.GetFileUploads(file.ID)
	if err != nil {
		fmt.Printf("  ❌ Could not fetch uploads for file %d: %v\n", file.ID, err)
		orm.UpdateFileStatus(file.ID, database.FilePending)
		return nil
	}

	hostNames := sortHostNames(adapters.Adapters)
	allSuccessful := true
	anyUploaded := false

	for _, hostName := range hostNames {
		if ctx.Err() != nil {
			orm.UpdateFileStatus(file.ID, database.FilePending)
			return ctx.Err()
		}

		uploaded, err := handleHost(orm, wc, file, hostName, uploads)
		if err != nil {
			allSuccessful = false
		}
		if uploaded {
			anyUploaded = true
		}
	}

	if allSuccessful {
		orm.UpdateFileStatus(file.ID, database.FileDone)
		fmt.Printf("✔️  File %d done across all hosts.\n", file.ID)
	} else {
		orm.UpdateFileStatus(file.ID, database.FilePending)
		fmt.Printf("⚠️  Some hosts failed for file %d — marked back to pending.\n", file.ID)
	}

	if anyUploaded {
		fmt.Printf("⏰  Waiting %s before next file.\n", uploadCooldown)
		time.Sleep(uploadCooldown)
	}

	return nil
}

func handleHost(orm *database.Orm, wc *website.Client, file sqlc.File, hostName string, existingUploads []sqlc.UploadJob) (uploaded bool, err error) {
	uploadJob := findUploadJob(existingUploads, hostName)
	adapter := adapters.Adapters[hostName]

	if uploadJob != nil && uploadJob.Status == "DONE" {
		fmt.Printf("  ⏩ [%s] Already uploaded previously. Skipping.\n", hostName)
		return false, nil
	}

	existsOnWebsite, _ := wc.CheckFile(file.FilePath, hostName)
	if existsOnWebsite {
		if uploadJob != nil {
			orm.CompleteUpload(file.ID, "EXISTS")
		} else {
			orm.AddUpload(file.ID, database.UploadDone, hostName, "EXISTS", "")
		}
		fmt.Printf("  ⏩ [%s] Already exist on website. Skipping.\n", hostName)
		return false, nil
	}

	fmt.Printf("  🔄 [%s] Uploading file\n", hostName)

	slug, err := adapter.Upload(file.FilePath)
	if err != nil {
		if uploadJob == nil {
			orm.AddUpload(file.ID, database.UploadFailed, hostName, "", err.Error())
		} else {
			orm.FailUpload(file.ID, err.Error())
		}
		fmt.Printf("  ❌ [%s] Upload Failed: %s\n", hostName, err.Error())
		return false, err
	}

	if uploadJob == nil {
		_ = orm.AddUpload(file.ID, database.UploadDone, hostName, slug, "")
	} else {
		_ = orm.CompleteUpload(file.ID, slug)
	}

	fmt.Printf("  ✨ [%s] Complete! -> Slug: %s\n", hostName, slug)
	wc.ImportToWebsite(file.FilePath, hostName, slug)

	return true, nil
}

func findUploadJob(uploadJobs []sqlc.UploadJob, hostName string) *sqlc.UploadJob {
	var uploadJob *sqlc.UploadJob

	for _, u := range uploadJobs {
		if u.HostName == hostName {
			uploadJob = &u
			break
		}
	}

	return uploadJob
}

func sortHostNames(a map[string]adapters.Adapter) []string {
	names := make([]string, 0, len(a))
	for n := range a {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
