package database

import (
	"runtime"
	"strings"
	"time"

	"github.com/adiecho/oci-panel/internal/models"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB(dsn string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(withSQLitePragmas(dsn)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}

	if err := models.AutoMigrate(DB); err != nil {
		return err
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	// WAL 模式（见 withSQLitePragmas）允许多个读连接与单个写连接并发，
	// 配合 busy_timeout 在写冲突时等待而非立即报错。由此可放开此前
	// SetMaxOpenConns(1) 造成的「所有请求 + 后台任务争抢单连接」的全局串行化瓶颈。
	maxOpen := runtime.NumCPU() * 2
	if maxOpen < 4 {
		maxOpen = 4
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxOpen)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	return nil
}

// withSQLitePragmas 为 SQLite DSN 追加并发友好的 PRAGMA（调用方未自带 _pragma 时）。
// 关键项：
//   - journal_mode(WAL)：读写并发，不再相互阻塞；
//   - busy_timeout(5000)：写冲突时最多等待 5s 而非立即返回 "database is locked"；
//   - synchronous(NORMAL)：WAL 下兼顾安全与性能的常用档位；
//   - foreign_keys(1)：开启外键约束。
//
// 这些 pragma 通过 DSN 传入，可保证连接池中的每个连接都生效。
func withSQLitePragmas(dsn string) string {
	if strings.Contains(dsn, "_pragma=") {
		return dsn // 调用方已自定义 pragma，不覆盖
	}
	pragmas := "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)"
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + pragmas
}

func GetDB() *gorm.DB {
	return DB
}

// UpsertSysSetting 按 key 写入系统设置：已存在则更新 value，不存在则以随机 ID 新建。
// 统一此前散落在 sys_controller / passkey_controller / scheduler_service / telegram_service
// 的「First → Create 或 Update」重复 upsert 逻辑。查询与更新均以 key 为准，ID 仅作主键占位。
func UpsertSysSetting(key, value string) error {
	var setting models.SysSetting
	if err := DB.Where("key = ?", key).First(&setting).Error; err != nil {
		return DB.Create(&models.SysSetting{
			ID:    uuid.New().String(),
			Key:   key,
			Value: value,
		}).Error
	}
	return DB.Model(&setting).Update("value", value).Error
}
