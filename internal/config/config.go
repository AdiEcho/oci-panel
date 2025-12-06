package config

import (
	"log"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Server struct {
		Port string `toml:"port"`
	} `toml:"server"`
	Web struct {
		Account  string `toml:"account"`
		Password string `toml:"password"`
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

	return &cfg
}
