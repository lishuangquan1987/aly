package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"aly/server/ent"
	"aly/server/internal/pathcfg"

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
	// 优先使用 DBPath，否则由 pathcfg 统一解析（A7：路径推导集中化）
	dbPath := DBPath
	if dbPath == "" {
		p, err := pathcfg.DBPath()
		if err != nil {
			log.Fatalf("failed to resolve database path: %v", err)
		}
		dbPath = p
	}
	// 确保数据库文件所在目录存在
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		log.Fatalf("failed to create database directory: %v", err)
	}
	// DSN 参数（审查 M4）：modernc.org/sqlite 只识别 _pragma，旧写法 "_fk=1"
	// 是 mattn/go-sqlite3 的参数，被静默忽略；外键仅在 Open 后靠一次性 PRAGMA
	// 开启，连接重建即失效。改用 _pragma 后驱动会在**每个新连接**重放。
	dsn := dbPath + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("failed opening connection to sqlite: %v", err)
	}
	// SQLite 单写入者限制：防止并发写入导致 "database is locked"。
	// WAL 模式下读不阻塞写，配合 busy_timeout 减少锁冲突；SetMaxOpenConns(1)
	// 仍确保同一时间只有一个连接，避免多连接下 SQLite 锁升级。
	db.SetMaxOpenConns(1)
	// WAL + 忙碌等待：并发读写更稳（WAL 允许读与写并发，崩溃可恢复）
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		log.Printf("failed enabling WAL mode (non-fatal): %v", err)
	}
	// 显式 PRAGMA 保留（与 DSN 双保险，便于阅读与排障）
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		log.Printf("failed setting busy_timeout (non-fatal): %v", err)
	}
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
	// 迁移：为已有数据库补充 project_id 列（v2 从 project_change_logs 重命名为 project_id）
	if err := migrateProjectChangeLogFK(db); err != nil {
		log.Fatalf("failed migrating project_change_logs FK: %v", err)
	}
}

// migrateProjectChangeLogFK 将 project_change_logs 表的旧 FK 列名 project_change_logs 迁移为 project_id
func migrateProjectChangeLogFK(database *sql.DB) error {
	// 查询 project_change_logs 表已有列
	rows, err := database.Query(`PRAGMA table_info('project_change_logs')`)
	if err != nil {
		return fmt.Errorf("query table_info: %w", err)
	}
	defer rows.Close()

	var hasProjectID, hasOldFK bool
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			log.Printf("migrateProjectChangeLogFK: failed to scan table_info row: %v", err)
			continue
		}
		if name == "project_id" {
			hasProjectID = true
		}
		if name == "project_change_logs" {
			hasOldFK = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table_info: %w", err)
	}

	// 已有 project_id 列：检查是否有未迁移的数据（上次迁移可能中断）
	if hasProjectID {
		if hasOldFK {
			var nullCount int
			if err := database.QueryRow(`SELECT COUNT(*) FROM project_change_logs WHERE project_id IS NULL AND project_change_logs IS NOT NULL`).Scan(&nullCount); err != nil {
				return fmt.Errorf("check unmigrated rows: %w", err)
			}
			if nullCount > 0 {
				log.Printf("migrateProjectChangeLogFK: found %d unmigrated rows, retrying data copy", nullCount)
				tx, err := database.Begin()
				if err != nil {
					return fmt.Errorf("begin retry tx: %w", err)
				}
				if _, err := tx.Exec(`UPDATE project_change_logs SET project_id = project_change_logs WHERE project_id IS NULL`); err != nil {
					tx.Rollback()
					return fmt.Errorf("copy remaining data: %w", err)
				}
				if err := tx.Commit(); err != nil {
					return fmt.Errorf("commit retry tx: %w", err)
				}
			}
			// 数据迁移完毕，尝试删除旧列
			dropOldColumn(database)
		}
		return nil
	}

	// 在事务中执行迁移以确保原子性
	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer tx.Rollback()

	// 添加 project_id 列（含外键约束）
	if _, err := tx.Exec(`ALTER TABLE project_change_logs ADD COLUMN project_id INTEGER REFERENCES projects(id) ON DELETE SET NULL`); err != nil {
		return fmt.Errorf("add project_id column: %w", err)
	}

	// 从旧列迁移数据
	if hasOldFK {
		if _, err := tx.Exec(`UPDATE project_change_logs SET project_id = project_change_logs`); err != nil {
			return fmt.Errorf("copy data: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration tx: %w", err)
	}
	// 迁移成功，清理旧列
	if hasOldFK {
		dropOldColumn(database)
	}
	return nil
}

// dropOldColumn 删除旧 FK 列 project_change_logs（SQLite ≥3.35 支持 DROP COLUMN）
func dropOldColumn(database *sql.DB) {
	if _, err := database.Exec(`ALTER TABLE project_change_logs DROP COLUMN project_change_logs`); err != nil {
		log.Printf("migrateProjectChangeLogFK: failed to drop old column (non-fatal, may need manual cleanup): %v", err)
	} else {
		log.Println("migrateProjectChangeLogFK: old column project_change_logs dropped")
	}
}
