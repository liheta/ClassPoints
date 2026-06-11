package app

import (
	"database/sql"
	"log"
	"strings"
	"time"
)

type sqlLogger struct {
	db      *sql.DB
	enabled bool
}

type dbResult interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

func newSQLLogger(db *sql.DB, enabled bool) *sqlLogger {
	return &sqlLogger{db: db, enabled: enabled}
}

type txLogger struct {
	tx      *sql.Tx
	enabled bool
}

func newTxLogger(tx *sql.Tx, enabled bool) *txLogger {
	return &txLogger{tx: tx, enabled: enabled}
}

func (l *sqlLogger) Exec(query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := l.db.Exec(query, args...)
	l.write("EXEC", query, args, time.Since(start), err)
	return result, err
}

func (l *sqlLogger) Query(query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := l.db.Query(query, args...)
	l.write("QUERY", query, args, time.Since(start), err)
	return rows, err
}

func (l *sqlLogger) QueryRow(query string, args ...any) *sql.Row {
	start := time.Now()
	row := l.db.QueryRow(query, args...)
	l.write("QUERY_ROW", query, args, time.Since(start), nil)
	return row
}

func (l *sqlLogger) write(kind string, query string, args []any, elapsed time.Duration, err error) {
	if !l.enabled {
		return
	}
	cleanQuery := strings.Join(strings.Fields(query), " ")
	if err != nil {
		log.Printf("[SQL] %s %s args=%v elapsed=%s error=%v", kind, cleanQuery, args, elapsed, err)
		return
	}
	log.Printf("[SQL] %s %s args=%v elapsed=%s", kind, cleanQuery, args, elapsed)
}

func (l *txLogger) Exec(query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := l.tx.Exec(query, args...)
	writeSQLLog(l.enabled, "TX_EXEC", query, args, time.Since(start), err)
	return result, err
}

func (l *txLogger) QueryRow(query string, args ...any) *sql.Row {
	start := time.Now()
	row := l.tx.QueryRow(query, args...)
	writeSQLLog(l.enabled, "TX_QUERY_ROW", query, args, time.Since(start), nil)
	return row
}

func writeSQLLog(enabled bool, kind string, query string, args []any, elapsed time.Duration, err error) {
	if !enabled {
		return
	}
	cleanQuery := strings.Join(strings.Fields(query), " ")
	if err != nil {
		log.Printf("[SQL] %s %s args=%v elapsed=%s error=%v", kind, cleanQuery, args, elapsed, err)
		return
	}
	log.Printf("[SQL] %s %s args=%v elapsed=%s", kind, cleanQuery, args, elapsed)
}
