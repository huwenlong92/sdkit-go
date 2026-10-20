package database

import (
	"context"

	"github.com/huwenlong92/sdkit-go/core/runtime"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

const KeyDatabase runtime.Key = "database"

func From(app *runtime.App) *Database {
	if app != nil {
		if value, ok := app.Container().Get(KeyDatabase); ok {
			if db, ok := value.(*Database); ok {
				return db
			}
		}
		return nil
	}
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return Default
}

func Bind(app *runtime.App, db *Database) error {
	if app == nil {
		setDefault(db)
		return nil
	}
	if db == nil {
		return runtime.ErrContainerValueNil
	}
	return runtime.BindProcessDefault(app, KeyDatabase, db, func() { setDefault(db) }, func() { setDefault(nil) })
}

func GormFrom(app *runtime.App) *gorm.DB {
	if db := From(app); db != nil {
		return db.gormHandle()
	}
	if app != nil {
		return nil
	}
	return nil
}

func Gorm(ctxs ...context.Context) *gorm.DB {
	db := GormFrom(nil)
	if db == nil {
		return nil
	}
	ctx := context.Background()
	if len(ctxs) > 0 && ctxs[0] != nil {
		ctx = ctxs[0]
	}
	return db.WithContext(ctx)
}

func PGXFrom(app *runtime.App) *pgxpool.Pool {
	if db := From(app); db != nil {
		return db.pgxHandle()
	}
	if app != nil {
		return nil
	}
	return nil
}

func PGX(ctxs ...context.Context) *pgxpool.Pool {
	_ = ctxs
	return PGXFrom(nil)
}

func Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	if fn == nil {
		return nil
	}
	db := Gorm(ctx)
	if db == nil {
		return ErrNotInitialized
	}
	return db.Transaction(fn)
}

func PGXTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	if fn == nil {
		return nil
	}
	pool := PGX(ctx)
	if pool == nil {
		return ErrNotInitialized
	}
	if ctx == nil {
		ctx = context.Background()
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func setDefault(db *Database) {
	var gormDB *gorm.DB
	var pool *pgxpool.Pool
	if db != nil {
		gormDB = db.gormHandle()
		pool = db.pgxHandle()
	}
	defaultMu.Lock()
	defer defaultMu.Unlock()
	Default = db
	DB = gormDB
	PGXPool = pool
}

func clearDefault(db *Database) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if Default == db {
		Default = nil
		DB = nil
		PGXPool = nil
	}
}
