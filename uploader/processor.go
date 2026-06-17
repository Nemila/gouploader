package uploader

import (
	"context"
	"database/sql"
	"errors"
	"gouploader/adapters"
	"gouploader/config"
	"gouploader/database"
	"gouploader/sqlc"
	"gouploader/website"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const uploadCooldown = 15 * time.Second

func ProcessFiles(log *slog.Logger, cfg *config.Config, ctx context.Context, orm *database.Orm) error {
	files, err := orm.Queries.GetFilesByStatus(ctx, "pending")
	if err != nil {
		return err
	}
	log.Info("Processing files...", "found", len(files))

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
		if err := processFile(log, ctx, orm, wc, file); err != nil {
			return err
		}
	}
	return nil
}

func processFile(log *slog.Logger, ctx context.Context, orm *database.Orm, wc *website.Client, file sqlc.File) error {
	log = log.With("fileName", filepath.Base(file.FilePath))
	log.Info("Processing file")

	if err := orm.Queries.UpsertFile(ctx, sqlc.UpsertFileParams{
		FilePath: file.FilePath,
		Status:   "processing",
		Archived: file.Archived,
	}); err != nil {
		log.Error("Failed to upsert file", "err", err)
	}

	uploadJobs, err := orm.Queries.GetFileUploads(ctx, file.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Error("Failed to fetch upload jobs", "err", err)
		if err := orm.Queries.UpsertFile(ctx, sqlc.UpsertFileParams{
			FilePath: file.FilePath,
			Status:   "pending",
			Archived: file.Archived,
		}); err != nil {
			log.Error("Failed to upsert file", "err", err)
		}
		return nil
	}

	hostNames := sortHostNames(adapters.Adapters)
	allSuccessful := true
	anyUploaded := false
	var mu sync.Mutex

	eg, egCtx := errgroup.WithContext(ctx)
	for _, hostName := range hostNames {
		func(hostName string) {
			eg.Go(func() error {
				uploadAttempted, err := handleHost(log, egCtx, orm, wc, file, hostName, uploadJobs)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					allSuccessful = false
				}
				if uploadAttempted && hostName != "hydrax" {
					anyUploaded = true
				}
				return nil
			})
		}(hostName)
	}
	eg.Wait()

	if allSuccessful {
		if err := orm.Queries.UpsertFile(ctx, sqlc.UpsertFileParams{
			FilePath: file.FilePath,
			Status:   "done",
			Archived: file.Archived,
		}); err != nil {
			log.Error("Failed to upsert file.")
		}
		log.Info("File uploaded across all hosts.")
	} else {
		if err := orm.Queries.UpsertFile(ctx, sqlc.UpsertFileParams{
			FilePath: file.FilePath,
			Status:   "pending",
			Archived: file.Archived,
		}); err != nil {
			log.Error("Failed to upsert file.")
		}
		log.Warn("Upload failed for some hosts.")
	}

	if anyUploaded {
		log.Info("Waiting before next file", "time", uploadCooldown)
		select {
		case <-time.After(uploadCooldown):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func handleHost(
	log *slog.Logger,
	ctx context.Context,
	orm *database.Orm,
	wc *website.Client,
	file sqlc.File,
	hostName string,
	existingUploads []sqlc.UploadJob,
) (uploadAttempted bool, err error) {
	log = log.With("hostName", hostName)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	uploadJob := findUploadJob(existingUploads, hostName)
	adapter := adapters.Adapters[hostName]

	if uploadJob != nil && uploadJob.Status == "done" {
		log.Info("File already uploaded, skipping.")
		return false, nil
	}

	if existsOnWebsite, err := wc.CheckFile(file.FilePath, hostName); existsOnWebsite {
		if err != nil {
			log.Error("Failed to check file on website.")
		}

		if err := orm.Queries.UpsertUpload(ctx, sqlc.UpsertUploadParams{
			FileID:   file.ID,
			HostName: hostName,
			Status:   "done",
			Slug: sql.NullString{
				String: "",
				Valid:  false,
			},
			LastError: sql.NullString{
				String: "",
				Valid:  false,
			},
		}); err != nil {
			log.Error("Failed to upsert upload", "err", err)
		}
		log.Info("File already exists on website")
		return false, nil
	}

	log.Info("Uploading file")

	slug, err := adapter.Upload(file.FilePath)
	if err != nil {
		if err := orm.Queries.UpsertUpload(ctx, sqlc.UpsertUploadParams{
			FileID:   file.ID,
			HostName: hostName,
			Status:   "failed",
			LastError: sql.NullString{
				String: err.Error(),
				Valid:  true,
			},
			Slug: sql.NullString{
				String: "",
				Valid:  false,
			},
		}); err != nil {
			log.Error("Failed to upsert upload")
		}

		log.Error("Failed to upload file", "err", err)
		return true, err
	}

	if err := orm.Queries.UpsertUpload(ctx, sqlc.UpsertUploadParams{
		FileID:   file.ID,
		HostName: hostName,
		Status:   "done",
		Slug: sql.NullString{
			String: slug,
			Valid:  true,
		},
		LastError: sql.NullString{
			String: "",
			Valid:  false,
		},
	}); err != nil {
		log.Error("Failed to upsert upload", "err", err)
	}

	log.Info("Complete!", "slug", slug)

	if err := wc.ImportToWebsite(log, file.FilePath, hostName, slug); err != nil {
		log.Error("Failed to import to website", "err", err)
	}
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
