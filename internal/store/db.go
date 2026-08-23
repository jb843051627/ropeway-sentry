// Package store 封装 SQLite 磁盘库的连接管理与全部持久化操作。
// 单写者约束：MaxOpenConns 恒为 1，杜绝并发写冲突。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store 持有唯一数据库连接句柄。
type Store struct {
	db *sql.DB
}

// Open 打开磁盘 SQLite 文件并完成建表。
// 显式拒绝 :memory: 数据源；MaxOpenConns 固定为 1。
func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("sqlite path must not be empty")
	}
	if strings.Contains(path, ":memory:") {
		return nil, fmt.Errorf("in-memory sqlite is not allowed: %s", path)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	s := &Store{db: db}
	if err := s.pingWithRetry(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return s, nil
}

// pingWithRetry modernc 驱动首次建立文件时偶发锁竞争，短退避重试。
func (s *Store) pingWithRetry() error {
	var err error
	for i := 0; i < 5; i++ {
		if err = s.db.Ping(); err == nil {
			return nil
		}
		time.Sleep(time.Duration(i+1) * 50 * time.Millisecond)
	}
	return fmt.Errorf("ping sqlite: %w", err)
}

// Close 关闭底层连接。
func (s *Store) Close() error { return s.db.Close() }

// DB 暴露原始句柄，仅供包内聚合查询复用场景之外的特殊用途（当前无调用方）。
func (s *Store) DB() *sql.DB { return s.db }

// Transaction 在单连接上执行事务函数；出错回滚并包装错误。
func (s *Store) Transaction(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// formatTime 统一 UTC RFC3339Nano 文本存储。
func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

// parseTime 从文本恢复 UTC 时间，解析失败返回零值由调用方兜底。
func parseTime(raw string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// nullTimePtr 把可空时间文本转为 *time.Time。
func nullTimePtr(raw sql.NullString) *time.Time {
	if !raw.Valid || raw.String == "" {
		return nil
	}
	t := parseTime(raw.String)
	return &t
}
