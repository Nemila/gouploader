package uploader

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gouploader/adapters"
	"gouploader/config"
	"gouploader/database"
	"gouploader/sqlc"
	"gouploader/website"
	"net/http"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const uploadCooldown = 2 * time.Minute

func ProcessFiles(cfg *config.Config, ctx context.Context, orm *database.Orm) error {
	fmt.Println("processing files")

	files, err := orm.Queries.GetFilesByStatus(ctx, "pending")
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

	if err := orm.Queries.UpsertFile(ctx, sqlc.UpsertFileParams{
		FilePath: file.FilePath,
		Status:   "processing",
	}); err != nil {
		fmt.Printf("\tfailed to upsert file %d: %v\n", file.ID, err)
	}

	uploads, err := orm.Queries.GetFileUploads(ctx, file.ID)
	if err != nil && !errors.Is(sql.ErrNoRows, err) {
		fmt.Printf("\tfailed to fetch uploads for file %d: %v\n", file.ID, err)
		orm.Queries.UpsertFile(ctx, sqlc.UpsertFileParams{
			FilePath: file.FilePath,
			Status:   "pending",
		})
		return nil
	}

	hostNames := sortHostNames(adapters.Adapters)
	allSuccessful := true
	anyUploaded := false
	var mu sync.Mutex
	g, groupCtx := errgroup.WithContext(ctx)

	for _, hostName := range hostNames {
		g.Go(func() error {
			uploaded, err := handleHost(groupCtx, orm, wc, file, hostName, uploads)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				allSuccessful = false
			}
			if uploaded {
				anyUploaded = true
			}
			return nil
		})

	}
	if err := g.Wait(); err != nil {
		return err
	}

	if allSuccessful {
		orm.Queries.UpsertFile(ctx, sqlc.UpsertFileParams{
			FilePath: file.FilePath,
			Status:   "done",
		})
		fmt.Printf("\tfile uploaded across all hosts.\n")
	} else {
		orm.Queries.UpsertFile(ctx, sqlc.UpsertFileParams{
			FilePath: file.FilePath,
			Status:   "pending",
		})
		fmt.Printf("\tfile upload failed for some hosts.\n")
	}

	if anyUploaded {
		fmt.Printf("\twaiting %s before next file.\n", uploadCooldown)
		time.Sleep(uploadCooldown)
	}

	return nil
}

func handleHost(ctx context.Context, orm *database.Orm, wc *website.Client, file sqlc.File, hostName string, existingUploads []sqlc.UploadJob) (uploaded bool, err error) {
	uploadJob := findUploadJob(existingUploads, hostName)
	adapter := adapters.Adapters[hostName]

	if uploadJob != nil && uploadJob.Status == "done" {
		fmt.Printf("\t[%s] already uploaded previously.\n", hostName)
		return false, nil
	}

	if existsOnWebsite, _ := wc.CheckFile(file.FilePath, hostName); existsOnWebsite {
		err := orm.Queries.UpsertUpload(ctx, sqlc.UpsertUploadParams{
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
		})
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("\t[%s] already exist on website.\n", hostName)
		return false, nil
	}

	fmt.Printf("\t[%s] uploading file %d\n", hostName, file.ID)

	slug, err := adapter.Upload(file.FilePath)
	if err != nil {
		orm.Queries.UpsertUpload(ctx, sqlc.UpsertUploadParams{
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
		})
		fmt.Printf("\t[%s] upload failed: %s\n", hostName, err.Error())
		return false, err
	}

	orm.Queries.UpsertUpload(ctx, sqlc.UpsertUploadParams{
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
	})

	fmt.Printf("\t[%s] Complete! -> Slug: %s\n", hostName, slug)
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
