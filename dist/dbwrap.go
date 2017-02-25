package dist

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"log"
	"time"
)

type DBWrap interface {
	BeginTx(context.Context, *sql.TxOptions) (TXWrap, error)
	Close() error
	Driver() driver.Driver
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PingContext(ctx context.Context) error
	// Prepare(query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	SetConnMaxLifetime(d time.Duration)
	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
	Stats() sql.DBStats
}

type dbWrap struct {
	log     *log.Logger
	backend *sql.DB
}

func (db *dbWrap) BeginTx(ctx context.Context, o *sql.TxOptions) (TXWrap, error) {
	tx, err := db.backend.BeginTx(ctx, o)
	if err != nil {
		return nil, err
	}
	db.log.Printf("SQL:BeginTx")
	return &txWrap{
		backend: tx,
		log:     db.log,
	}, nil
}

func (db *dbWrap) Close() error {
	return db.backend.Close()
}

func (db *dbWrap) Driver() driver.Driver {
	return db.backend.Driver()
}

func (db *dbWrap) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	db.log.Printf("SQL:Exec> %q %q", query, args)
	return db.backend.ExecContext(ctx, query, args...)
}

func (db *dbWrap) PingContext(ctx context.Context) error {
	db.log.Printf("SQL:Ping")
	return db.backend.PingContext(ctx)
}

func (db *dbWrap) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	// TODO: not implemented.
	return db.backend.PrepareContext(ctx, query)
}

func (db *dbWrap) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	db.log.Printf("SQL:Query> %q %q", query, args)
	return db.backend.QueryContext(ctx, query, args...)
}

func (db *dbWrap) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	db.log.Printf("SQL:QueryRow> %q %q", query, args)
	return db.backend.QueryRowContext(ctx, query, args...)
}

func (db *dbWrap) SetConnMaxLifetime(d time.Duration) {
	db.backend.SetConnMaxLifetime(d)
}

func (db *dbWrap) SetMaxIdleConns(n int) {
	db.backend.SetMaxIdleConns(n)
}

func (db *dbWrap) SetMaxOpenConns(n int) {
	db.backend.SetMaxOpenConns(n)
}

func (db *dbWrap) Stats() sql.DBStats {
	return db.backend.Stats()
}

type TXWrap interface {
	Commit() error
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	//Prepare(query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	Rollback() error
	//Stmt(stmt *sql.Stmt) *sql.Stmt
}

type txWrap struct {
	log     *log.Logger
	backend *sql.Tx
}

func (tx *txWrap) Commit() error {
	tx.log.Printf("SQL:TX:Commit")
	return tx.backend.Commit()
}

func (tx *txWrap) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	tx.log.Printf("SQL:TX:Exec> %q %q", query, args)
	return tx.backend.ExecContext(ctx, query, args...)
}

func (tx *txWrap) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	// TODO: not implemented.
	return tx.backend.PrepareContext(ctx, query)
}

func (tx *txWrap) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	tx.log.Printf("SQL:TX:Query> %q %q", query, args)
	return tx.backend.QueryContext(ctx, query, args...)
}

func (tx *txWrap) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	tx.log.Printf("SQL:TX:QueryRow> %q %q", query, args)
	return tx.backend.QueryRowContext(ctx, query, args...)
}

func (tx *txWrap) Rollback() error {
	tx.log.Printf("SQL:TX:Rollback")
	return tx.backend.Rollback()
}

func (tx *txWrap) StmtContext(ctx context.Context, stmt *sql.Stmt) *sql.Stmt {
	// TODO: not implemented.
	return tx.backend.StmtContext(ctx, stmt)
}

func NewDBWrap(db *sql.DB, logger *log.Logger) DBWrap {
	return &dbWrap{
		backend: db,
		log:     logger,
	}
}
