package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"gouploader/sqlc"
)

type Orm struct {
	ctx     context.Context
	db      *sql.DB
	queries *sqlc.Queries
}

//go:embed schema.sql
var schema string

type FileStatus int

const (
	FilePending FileStatus = iota
	FileProcessing
	FileMissing
	FileDone
)

type UploadStatus int

const (
	UploadPending UploadStatus = iota
	UploadFailed
	UploadDone
)

func NewOrm() (*Orm, error) {
	ctx := context.Background()

	db, err := sql.Open("sqlite", "./database.db")
	if err != nil {
		return nil, fmt.Errorf("[o.NewOrm] failed to open database: %w", err)
	}

	queries := sqlc.New(db)

	return &Orm{
		ctx:     ctx,
		db:      db,
		queries: queries,
	}, nil
}

func (o *Orm) InitDatabase() error {
	tableExists := false
	query := `SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type='table' AND name='files');`

	err := o.db.QueryRowContext(o.ctx, query).Scan(&tableExists)
	if err != nil {
		return fmt.Errorf("[o.Migrate] failed to check database state: %w", err)
	}

	if !tableExists {
		fmt.Println("🔄 New database detected. Running initial migration schema...")

		_, err = o.db.ExecContext(o.ctx, schema)
		if err != nil {
			return fmt.Errorf("[o.Migrate] failed to execute migration schema: %w", err)
		}
		fmt.Println("✅ Database successfully initialized!")
	}

	return nil
}

func (o *Orm) GetPendingFiles(page int64, perPage int64) ([]sqlc.File, error) {
	files, err := o.queries.GetPendingFile(o.ctx, sqlc.GetPendingFileParams{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	})
	if err != nil {
		return nil, fmt.Errorf("[o.GetPendingFiles] query failed: %w", err)
	}
	return files, nil
}

func (o *Orm) FindFileByPath(path string) (*sqlc.File, error) {
	file, err := o.queries.FindFileByPath(o.ctx, path)
	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return nil, nil
		}
		return nil, fmt.Errorf("[o.FindFileByPath] query failed: %w", err)
	}
	return &file, nil
}

func (o *Orm) GetFileUploads(fileId int64) ([]sqlc.UploadJob, error) {
	uploads, err := o.queries.GetFileUploads(o.ctx, fileId)
	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return nil, nil
		}
		return nil, fmt.Errorf("[o.GetFileUploads] query failed: %w", err)
	}
	return uploads, nil
}

func (o *Orm) RegisterFile(path string) error {
	fileExists, err := o.FindFileByPath(path)
	if err != nil {
		return fmt.Errorf("[o.RegisterFile] find file by path failed: %w", err)
	}

	if fileExists != nil {
		return nil
	}

	err = o.queries.AddFile(o.ctx, path)
	if err != nil {
		return fmt.Errorf("[o.RegisterFile] failed to add file: %w", err)
	}
	return nil
}

func (o *Orm) UpdateFileStatus(fileId int64, status FileStatus) error {
	statuses := [...]string{"PENDING", "PROCESSING", "MISSING", "DONE"}
	if status < FilePending || status > FileDone {
		return fmt.Errorf("[o.UpdateFileStatus] invalid status")
	}

	err := o.queries.UpdateFileStatus(o.ctx, sqlc.UpdateFileStatusParams{
		ID:     fileId,
		Status: statuses[status],
	})
	if err != nil {
		return fmt.Errorf("[o.UpdateFileStatus] query failed: %w", err)
	}

	return nil
}

func (o *Orm) FailUpload(fileId int64, errorMsg string) error {
	err := o.queries.FailUpload(o.ctx, sqlc.FailUploadParams{
		ID:        fileId,
		LastError: sql.NullString{String: errorMsg, Valid: errorMsg != ""},
	})
	if err != nil {
		return fmt.Errorf("[o.FailUpload] query failed: %w", err)
	}
	return nil
}

func (o *Orm) CompleteUpload(fileId int64, slug string) error {
	err := o.queries.CompleteUpload(o.ctx, sqlc.CompleteUploadParams{
		ID: fileId,
		Slug: sql.NullString{
			String: slug,
			Valid:  slug != "",
		},
	})
	if err != nil {
		return fmt.Errorf("[o.CompleteUpload] query failed: %w", err)
	}
	return nil
}

func (o *Orm) AddUpload(fileId int64, status UploadStatus, hostName string, slug string, errorMsg string) error {
	statuses := [...]string{"PENDING", "FAILED", "DONE"}
	if status < UploadPending || status > UploadDone {
		return fmt.Errorf("[o.AddUpload] invalid status")
	}

	err := o.queries.AddUpload(o.ctx, sqlc.AddUploadParams{
		Status:    statuses[status],
		FileID:    fileId,
		HostName:  hostName,
		Slug:      sql.NullString{String: slug, Valid: slug != ""},
		LastError: sql.NullString{String: errorMsg, Valid: errorMsg != ""},
	})
	if err != nil {
		return fmt.Errorf("[o.AddUpload] query failed: %w", err)
	}
	return nil
}

func (o *Orm) ResetProcessingStatuses() error {
	err := o.queries.ResetProcessingStatuses(o.ctx)
	if err != nil {
		return fmt.Errorf("[o.ResetProcessingStatuses] query failed: %w", err)
	}
	return nil
}
