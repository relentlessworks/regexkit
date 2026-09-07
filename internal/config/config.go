package config

import (
	"flag"
	"fmt"
	"os"
)

type Config struct {
	Addr   string
	DB     string
	Secret string
}

func Load() *Config {
	cfg := &Config{
		Addr:   ":7700",
		DB:     "regexkit.json",
		Secret: "",
	}

	if addr := os.Getenv("REGEXKIT_ADDR"); addr != "" {
		cfg.Addr = addr
	}
	if db := os.Getenv("REGEXKIT_DB"); db != "" {
		cfg.DB = db
	}
	if secret := os.Getenv("REGEXKIT_SECRET"); secret != "" {
		cfg.Secret = secret
	}

	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "listen address")
	flag.StringVar(&cfg.DB, "db", cfg.DB, "data file path")
	flag.StringVar(&cfg.Secret, "secret", cfg.Secret, "auth token signing secret")
	flag.Parse()

	return cfg
}

func (c *Config) String() string {
	return fmt.Sprintf("addr=%s db=%s", c.Addr, c.DB)
}
