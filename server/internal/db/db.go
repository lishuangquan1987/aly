package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"clientupdator/server/ent"

	sqldialect "entgo.io/ent/dialect/sql"

	_ "modernc.org/sqlite"
)

var Client *ent.Client

// DBPath 数据库文件路径，InitDB 前设置可自定义路径
var DBPath string

func WithTx(ctx context.Context, fn func(tx *ent.Tx) error) error {
	tx, err := Client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if v := recover(); v != nil {
			tx.Rollback()
			panic(v)
		}
	}()
	if err := fn(tx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			err = fmt.Errorf("%w: rolling back transaction: %v", err, rerr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

// InitDB 初始化数据库 — 数据库文件默认放在程序根目录（go run 时为工作目录）
func InitDB() {
	// 优先使用 DBPath，否则基于可执行文件路径解析（go run 时回退到工作目录）
	dbPath := DBPath
	if dbPath == "" {
		dbDir := ""
		exe, err := os.Executable()
		if err == nil {
			exeDir := filepath.Dir(exe)
			// go run 会将可执行文件放在临时目录，此时回退到工作目录
			if !strings.Contains(exeDir, os.TempDir()) {
				dbDir = exeDir
			}
		}
		if dbDir == "" {
			wd, err := os.Getwd()
			if err != nil {
				log.Fatalf("failed to get working directory: %v", err)
			}
			dbDir = wd
		}
		dbPath = filepath.Join(dbDir, "clientupdator.db")
	}
	// 确保数据库文件所在目录存在
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		log.Fatalf("failed to create database directory: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath+"?_fk=1")
	if err != nil {
		log.Fatalf("failed opening connection to sqlite: %v", err)
	}
	// SQLite 单写入者限制：防止并发写入导致 "database is locked"
	db.SetMaxOpenConns(1)
	// Enable foreign keys (required by modernc.org/sqlite)
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		log.Fatalf("failed enabling foreign keys: %v", err)
	}
	// Create a custom ent driver with sqlite3 dialect
	driver := sqldialect.OpenDB("sqlite3", db)
	Client = ent.NewClient(ent.Driver(driver))
	// Run the auto migration tool.
	if err := Client.Schema.Create(context.Background()); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}
}
