package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sync"

	"github.com/pressly/goose/v3"
)

const Dir = "sql/schema"

var configure = sync.OnceValue(func() error {
	goose.SetLogger(goose.NopLogger())
	return goose.SetDialect("postgres")
})

type Result struct {
	From int64
	To   int64
}

func (r Result) Applied() int64 { return r.To - r.From }

func Up(ctx context.Context, db *sql.DB, schema fs.FS) (Result, error) {
	if err := prepare(schema); err != nil {
		return Result{}, err
	}

	from, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return Result{}, fmt.Errorf("reading schema version: %w", err)
	}

	if err := goose.UpContext(ctx, db, Dir); err != nil {
		return Result{}, fmt.Errorf("applying migrations: %w", err)
	}

	to, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return Result{}, fmt.Errorf("reading schema version: %w", err)
	}
	return Result{From: from, To: to}, nil
}

func Version(ctx context.Context, db *sql.DB, schema fs.FS) (int64, error) {
	if err := prepare(schema); err != nil {
		return 0, err
	}

	v, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return 0, fmt.Errorf("reading schema version: %w", err)
	}
	return v, nil
}

func Pending(ctx context.Context, db *sql.DB, schema fs.FS) ([]string, error) {
	if err := prepare(schema); err != nil {
		return nil, err
	}

	current, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("reading schema version: %w", err)
	}

	all, err := goose.CollectMigrations(Dir, 0, goose.MaxVersion)
	if err != nil {
		return nil, fmt.Errorf("collecting migrations: %w", err)
	}

	var pending []string
	for _, m := range all {
		if m.Version > current {
			pending = append(pending, m.Source)
		}
	}
	return pending, nil
}

func prepare(schema fs.FS) error {
	if schema == nil {
		return fmt.Errorf("no embedded schema")
	}
	if err := configure(); err != nil {
		return fmt.Errorf("configuring goose: %w", err)
	}
	goose.SetBaseFS(schema)
	return nil
}
