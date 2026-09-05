package config

import (
	"log"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Server struct {
		Port string `toml:"port"`
	} `toml:"server"`
	Web struct {
		Account   string `toml:"account"`
		Password  string `toml:"password"`
		JwtSecret string `toml:"jwt_secret"`
		// AllowOrigins 是允许跨域访问的来源白名单（精确匹配 Origin 头）。
		// 留空表示仅允许同源访问（生产环境前端由本服务同源托管，无需配置）。
		AllowOrigins []string `toml:"allow_origins"`
	} `toml:"web"`
	Database struct {
		DSN string `toml:"dsn"`
	} `toml:"database"`
	Logging struct {
		Level string `toml:"level"`
	} `toml:"logging"`
	Passkey struct {
		RPID      string   `toml:"rp_id"`
		RPOrigins []string `toml:"rp_origins"`
	} `toml:"passkey"`
}

func Load() *Config {
	data, err := os.ReadFile("config.toml")
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}
	if strings.TrimSpace(cfg.Web.Account) == "" || strings.TrimSpace(cfg.Web.Password) == "" {
		log.Fatalf("web.account and web.password must be configured")
	}
	if cfg.Web.Account == "admin" && cfg.Web.Password == "admin" {
		log.Fatalf("the default admin/admin credentials are not allowed")
	}
	for _, origin := range cfg.Web.AllowOrigins {
		if strings.TrimSpace(origin) == "*" {
			log.Fatalf("web.allow_origins must not contain '*' when authentication is enabled")
		}
	}

	return &cfg
}
