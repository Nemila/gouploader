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
		return nil, fmt.Errorf("[orm.NewOrm] failed to open database: %w", err)
	}

	queries := sqlc.New(db)
	return &Orm{
		ctx:     ctx,
		db:      db,
		queries: queries,
	}, nil
}

func (orm *Orm) Migrate() error {
	_, err := orm.db.ExecContext(orm.ctx, schema)
	if err != nil {
		return fmt.Errorf("[orm.Migrate] failed execute schema: %w", err)
	}
	return nil
}

func (orm *Orm) GetPendingFiles(page int64, perPage int64) ([]sqlc.File, error) {
	files, err := orm.queries.GetPendingFile(orm.ctx, sqlc.GetPendingFileParams{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	})
	if err != nil {
		return nil, fmt.Errorf("[orm.GetPendingFiles] query failed: %w", err)
	}
	return files, nil
}

func (orm *Orm) FindFileByPath(path string) (*sqlc.File, error) {
	file, err := orm.queries.FindFileByPath(orm.ctx, path)
	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return nil, nil
		}
		return nil, fmt.Errorf("[orm.FindFileByPath] query failed: %w", err)
	}
	return &file, nil
}

func (orm *Orm) GetFileUploads(fileId int64) ([]sqlc.UploadJob, error) {
	uploads, err := orm.queries.GetFileUploads(orm.ctx, fileId)
	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return nil, nil
		}
		return nil, fmt.Errorf("[orm.GetFileUploads] query failed: %w", err)
	}
	return uploads, nil
}

func (orm *Orm) RegisterFile(path string) error {
	fileExists, err := orm.FindFileByPath(path)
	if err != nil {
		return fmt.Errorf("[orm.RegisterFile] find file by path failed: %w", err)
	}

	if fileExists != nil {
		return fmt.Errorf("[orm.RegisterFile] file already exists: %w", err)
	}

	err = orm.queries.AddFile(orm.ctx, path)
	if err != nil {
		return fmt.Errorf("[orm.RegisterFile] failed to add file: %w", err)
	}
	return nil
}

func (orm *Orm) UpdateFileStatus(fileId int64, status FileStatus, errorMsg string) error {
	statuses := [...]string{"PENDING", "PROCESSING", "MISSING", "DONE"}
	if status < FilePending || status > FileDone {
		return fmt.Errorf("[orm.UpdateFileStatus] invalid status")
	}

	err := orm.queries.UpdateFileStatus(orm.ctx, sqlc.UpdateFileStatusParams{
		ID:     fileId,
		Status: statuses[status],
		Error:  sql.NullString{String: errorMsg, Valid: errorMsg != ""},
	})
	if err != nil {
		return fmt.Errorf("[orm.UpdateFileStatus] query failed: %w", err)
	}

	return nil
}

func (orm *Orm) FailUpload(fileId int64, errorMsg string) error {
	err := orm.queries.FailUpload(orm.ctx, sqlc.FailUploadParams{
		ID:        fileId,
		LastError: sql.NullString{String: errorMsg, Valid: errorMsg != ""},
	})
	if err != nil {
		return fmt.Errorf("[orm.FailUpload] query failed: %w", err)
	}
	return nil
}

func (orm *Orm) CompleteUpload(fileId int64, slugId string) error {
	err := orm.queries.CompleteUpload(orm.ctx, sqlc.CompleteUploadParams{
		ID: fileId,
		SlugID: sql.NullString{
			String: slugId,
			Valid:  slugId != "",
		},
	})
	if err != nil {
		return fmt.Errorf("[orm.CompleteUpload] query failed: %w", err)
	}
	return nil
}

func (orm *Orm) AddUpload(fileId int64, status UploadStatus, hostName string, slugId string, errorMsg string) error {
	statuses := [...]string{"PENDING", "FAILED", "DONE"}
	if status < UploadPending || status > UploadDone {
		return fmt.Errorf("[orm.AddUpload] invalid status")
	}

	err := orm.queries.AddUpload(orm.ctx, sqlc.AddUploadParams{
		Status:    statuses[status],
		FileID:    fileId,
		HostName:  hostName,
		SlugID:    sql.NullString{String: slugId, Valid: slugId != ""},
		LastError: sql.NullString{String: errorMsg, Valid: errorMsg != ""},
	})
	if err != nil {
		return fmt.Errorf("[orm.AddUpload] query failed: %w", err)
	}
	return nil
}
