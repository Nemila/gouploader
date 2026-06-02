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
	PENDING FileStatus = iota
	PROCESSING
	MISSING
	DONE
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

func (orm *Orm) UpdateFileStatus(status FileStatus, errorMsg string, fileId int64) error {
	statuses := [...]string{"PENDING", "PROCESSING", "MISSING", "DONE"}
	err := orm.queries.UpdateFileStatus(orm.ctx, sqlc.UpdateFileStatusParams{
		Status: sql.NullString{String: statuses[status], Valid: status >= PENDING || status <= DONE},
		Error:  sql.NullString{String: errorMsg, Valid: errorMsg != ""},
		ID:     fileId,
	})
	if err != nil {
		return fmt.Errorf("[orm.UpdateFileStatus] query failed: %w", err)
	}
	return nil
}
