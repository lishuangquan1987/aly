package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

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

func InitDB() {
	//初始化数据库 — 基于可执行文件路径解析 db 路径（允许通过 DBPath 变量自定义）
	dbPath := DBPath
	if dbPath == "" {
		exe, err := os.Executable()
		if err != nil {
			log.Fatalf("failed to get executable path: %v", err)
		}
		exeDir := filepath.Dir(exe)
		dbPath = filepath.Join(exeDir, "configs", "clientupdator.db")
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
