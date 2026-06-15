package database

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
		return nil, err
	}
	queries := sqlc.New(db)

	o := &Orm{
		ctx:     ctx,
		db:      db,
		queries: queries,
	}

	if err := o.InitDatabase(); err != nil {
		panic(err.Error())
	}

	return o, nil
}

func (o *Orm) InitDatabase() error {
	query := `SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type='table' AND name='files');`
	tableExists := false

	if err := o.db.QueryRowContext(o.ctx, query).Scan(&tableExists); err != nil {
		return err
	}

	if !tableExists {
		fmt.Println("🔄 New database detected. Running initial migration schema...")

		if _, err := o.db.ExecContext(o.ctx, schema); err != nil {
			return err
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
		return nil, err
	}
	return files, nil
}

func (o *Orm) FindFileByPath(path string) (*sqlc.File, error) {
	file, err := o.queries.FindFileByPath(o.ctx, path)
	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return nil, nil
		}
		return nil, err
	}
	return &file, nil
}

func (o *Orm) GetFileUploads(fileId int64) ([]sqlc.UploadJob, error) {
	uploads, err := o.queries.GetFileUploads(o.ctx, fileId)
	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return nil, nil
		}
		return nil, err
	}
	return uploads, nil
}

func (o *Orm) RegisterFile(path string) error {
	f, err := o.FindFileByPath(path)
	if err != nil {
		return err
	}

	if f != nil {
		return nil
	}

	if err := o.queries.AddFile(o.ctx, path); err != nil {
		return err
	}
	return nil
}

func (o *Orm) UpdateFileStatus(fileId int64, status FileStatus) error {
	statuses := [...]string{"PENDING", "PROCESSING", "MISSING", "DONE"}
	if status < FilePending || status > FileDone {
		return fmt.Errorf("can't update file with invalid status")
	}

	if err := o.queries.UpdateFileStatus(o.ctx, sqlc.UpdateFileStatusParams{
		ID:     fileId,
		Status: statuses[status],
	}); err != nil {
		return err
	}

	return nil
}

func (o *Orm) FailUpload(fileId int64, errorMsg string) error {
	if err := o.queries.FailUpload(o.ctx, sqlc.FailUploadParams{
		ID:        fileId,
		LastError: sql.NullString{String: errorMsg, Valid: errorMsg != ""},
	}); err != nil {
		return err
	}
	return nil
}

func (o *Orm) CompleteUpload(fileId int64, slug string) error {
	if err := o.queries.CompleteUpload(o.ctx, sqlc.CompleteUploadParams{
		ID: fileId,
		Slug: sql.NullString{
			String: slug,
			Valid:  slug != "",
		},
	}); err != nil {
		return err
	}
	return nil
}

func (o *Orm) AddUpload(fileId int64, status UploadStatus, hostName string, slug string, errorMsg string) error {
	statuses := [...]string{"PENDING", "FAILED", "DONE"}
	if status < UploadPending || status > UploadDone {
		return fmt.Errorf("cant add upload with invalid status")
	}

	if err := o.queries.AddUpload(o.ctx, sqlc.AddUploadParams{
		Status:    statuses[status],
		FileID:    fileId,
		HostName:  hostName,
		Slug:      sql.NullString{String: slug, Valid: slug != ""},
		LastError: sql.NullString{String: errorMsg, Valid: errorMsg != ""},
	}); err != nil {
		return err
	}

	return nil
}

func (o *Orm) ResetProcessingStatuses() error {
	if err := o.queries.ResetProcessingStatuses(o.ctx); err != nil {
		return err
	}
	return nil
}
