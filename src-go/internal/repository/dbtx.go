package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// DBTX is the minimal query surface repositories depend on.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type gormDBTX struct {
	db *gorm.DB
}

func NewGormDBTX(db *gorm.DB) DBTX {
	if db == nil {
		return nil
	}
	return &gormDBTX{db: db}
}

func (g *gormDBTX) Exec(ctx context.Context, query string, arguments ...any) (pgconn.CommandTag, error) {
	result := g.db.WithContext(ctx).Exec(query, arguments...)
	if result.Error != nil {
		return pgconn.CommandTag{}, result.Error
	}
	return pgconn.NewCommandTag(fmt.Sprintf("OK %d", result.RowsAffected)), nil
}

func (g *gormDBTX) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	rows, err := g.db.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	return &gormRows{rows: rows}, nil
}

func (g *gormDBTX) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return &gormRow{row: g.db.WithContext(ctx).Raw(query, args...).Row()}
}

func (g *gormDBTX) Begin(ctx context.Context) (pgx.Tx, error) {
	tx := g.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &gormTx{db: tx}, nil
}

type gormRow struct {
	row *sql.Row
}

func (r *gormRow) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return pgx.ErrNoRows
	}
	return err
}

type gormRows struct {
	rows *sql.Rows
}

func (r *gormRows) Close() {
	_ = r.rows.Close()
}

func (r *gormRows) Err() error {
	return r.rows.Err()
}

func (r *gormRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *gormRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *gormRows) Next() bool {
	return r.rows.Next()
}

func (r *gormRows) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

func (r *gormRows) Values() ([]any, error) {
	return nil, fmt.Errorf("gorm row values are not exposed")
}

func (r *gormRows) RawValues() [][]byte {
	return nil
}

func (r *gormRows) Conn() *pgx.Conn {
	return nil
}

type gormTx struct {
	db *gorm.DB
}

func (t *gormTx) Begin(ctx context.Context) (pgx.Tx, error) {
	tx := t.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &gormTx{db: tx}, nil
}

func (t *gormTx) Commit(ctx context.Context) error {
	return t.db.WithContext(ctx).Commit().Error
}

func (t *gormTx) Rollback(ctx context.Context) error {
	return t.db.WithContext(ctx).Rollback().Error
}

func (t *gormTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, fmt.Errorf("copy from is not supported by the gorm adapter")
}

func (t *gormTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return nil
}

func (t *gormTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (t *gormTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, fmt.Errorf("prepare is not supported by the gorm adapter")
}

func (t *gormTx) Exec(ctx context.Context, query string, arguments ...any) (pgconn.CommandTag, error) {
	return (&gormDBTX{db: t.db}).Exec(ctx, query, arguments...)
}

func (t *gormTx) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return (&gormDBTX{db: t.db}).Query(ctx, query, args...)
}

func (t *gormTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return (&gormDBTX{db: t.db}).QueryRow(ctx, query, args...)
}

func (t *gormTx) Conn() *pgx.Conn {
	return nil
}
