package database

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"gouploader/sqlc"
)

type Orm struct {
	Db      *sql.DB
	Queries *sqlc.Queries
}

//go:embed schema.sql
var schema string

func NewOrm(ctx context.Context) (*Orm, error) {
	db, err := sql.Open("sqlite", "./database.db?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	queries := sqlc.New(db)

	o := &Orm{
		Db:      db,
		Queries: queries,
	}
	if err := o.InitDatabase(ctx); err != nil {
		return nil, err
	}

	return o, nil
}

func (o *Orm) InitDatabase(ctx context.Context) error {
	query := `SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type='table' AND name='files');`
	tableExists := false

	if err := o.Db.QueryRowContext(ctx, query).Scan(&tableExists); err != nil {
		return err
	}

	if !tableExists {
		fmt.Println("🔄 New database detected. Running initial migration schema...")

		if _, err := o.Db.ExecContext(ctx, schema); err != nil {
			return err
		}
		fmt.Println("✅ Database successfully initialized!")
	}

	return nil
}
