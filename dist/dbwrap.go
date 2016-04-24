package dist

import (
	"database/sql"
	"database/sql/driver"
	"log"
	"time"
)

type DBWrap interface {
	Begin() (TXWrap, error)
	Close() error
	Driver() driver.Driver
	Exec(query string, args ...interface{}) (sql.Result, error)
	Ping() error
	// Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	SetConnMaxLifetime(d time.Duration)
	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
	Stats() sql.DBStats
}

type dbWrap struct {
	log     *log.Logger
	backend *sql.DB
}

func (db *dbWrap) Begin() (TXWrap, error) {
	tx, err := db.backend.Begin()
	if err != nil {
		return nil, err
	}
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

func (db *dbWrap) Exec(query string, args ...interface{}) (sql.Result, error) {
	db.log.Printf("SQL:Exec> %q %q", query, args)
	return db.backend.Exec(query, args...)
}

func (db *dbWrap) Ping() error {
	return db.backend.Ping()
}

func (db *dbWrap) Prepare(query string) (*sql.Stmt, error) {
	// TODO: not implemented.
	return db.backend.Prepare(query)
}

func (db *dbWrap) Query(query string, args ...interface{}) (*sql.Rows, error) {
	db.log.Printf("SQL:Query> %q %q", query, args)
	return db.backend.Query(query, args...)
}

func (db *dbWrap) QueryRow(query string, args ...interface{}) *sql.Row {
	db.log.Printf("SQL:QueryRow> %q %q", query, args)
	return db.backend.QueryRow(query, args...)
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
	Exec(query string, args ...interface{}) (sql.Result, error)
	//Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Rollback() error
	//Stmt(stmt *sql.Stmt) *sql.Stmt
}

type txWrap struct {
	log     *log.Logger
	backend *sql.Tx
}

func (tx *txWrap) Commit() error {
	return tx.backend.Commit()
}

func (tx *txWrap) Exec(query string, args ...interface{}) (sql.Result, error) {
	tx.log.Printf("SQL:Exec> %q %q", query, args)
	return tx.backend.Exec(query, args...)
}

func (tx *txWrap) Prepare(query string) (*sql.Stmt, error) {
	// TODO: not implemented.
	return tx.backend.Prepare(query)
}

func (tx *txWrap) Query(query string, args ...interface{}) (*sql.Rows, error) {
	tx.log.Printf("SQL:Query> %q %q", query, args)
	return tx.backend.Query(query, args...)
}

func (tx *txWrap) QueryRow(query string, args ...interface{}) *sql.Row {
	tx.log.Printf("SQL:QueryRow> %q %q", query, args)
	return tx.backend.QueryRow(query, args...)
}

func (tx *txWrap) Rollback() error {
	return tx.backend.Rollback()
}

func (tx *txWrap) Stmt(stmt *sql.Stmt) *sql.Stmt {
	// TODO: not implemented.
	return tx.backend.Stmt(stmt)
}

func NewDBWrap(db *sql.DB, logger *log.Logger) DBWrap {
	return &dbWrap{
		backend: db,
		log:     logger,
	}
}
