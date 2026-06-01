package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"gouploader/sqlc"
)

type Orm struct {
	ctx     context.Context
	db      *sql.DB
	queries *sqlc.Queries
}

//go:embed schema.sql
var schema string

func NewOrm() (*Orm, error) {
	ctx := context.Background()

	db, err := sql.Open("sqlite", "./database.db")
	if err != nil {
		return nil, err
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
		return err
	}
	return nil
}

func (orm *Orm) GetPendingFiles(page int64, perPage int64) ([]sqlc.File, error) {
	files, err := orm.queries.GetPendingFile(orm.ctx, sqlc.GetPendingFileParams{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (orm *Orm) FindFileByPath(path string) (*sqlc.File, error) {
	file, err := orm.queries.FindFileByPath(orm.ctx, path)
	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return nil, nil
		}
		return nil, err
	}
	return &file, nil
}

func (orm *Orm) GetFileUploads(fileId int64) ([]sqlc.UploadJob, error) {
	uploads, err := orm.queries.GetFileUploads(orm.ctx, fileId)
	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return nil, nil
		}
		return nil, err
	}
	return uploads, nil
}

func (orm *Orm) RegisterFile(path string) error {
	fileExists, err := orm.FindFileByPath(path)
	if err != nil {
		return err
	}

	if fileExists != nil {
		return nil
	}

	err = orm.queries.AddFile(orm.ctx, path)
	if err != nil {
		return err
	}
	return nil
}

type FileStatus int

const (
	PENDING FileStatus = iota
	PROCESSING
	MISSING
	DONE
)

func (orm *Orm) UpdateFileStatus(status FileStatus, errorMsg string, fileId int64) error {
	statuses := [...]string{"PENDING", "PROCESSING", "MISSING", "DONE"}
	err := orm.queries.UpdateFileStatus(orm.ctx, sqlc.UpdateFileStatusParams{
		Status: sql.NullString{String: statuses[status], Valid: status >= PENDING || status <= DONE},
		Error:  sql.NullString{String: errorMsg, Valid: errorMsg != ""},
		ID:     fileId,
	})
	if err != nil {
		return err
	}
	return nil
}
