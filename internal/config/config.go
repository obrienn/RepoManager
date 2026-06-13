package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	ServerPort     string
	ServerAddr     string
	LibraryPath    string
	GithubToken    string
	UpdateInterval string
}

func Load(path string) (*Config, error) {
	cfg := defaults()
	if err := loadEnv(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaults() *Config {
	return &Config{
		DBHost:      "localhost",
		DBPort:      "5432",
		DBUser:      "repoman",
		DBPassword:  "",
		DBName:      "repoman",
		ServerPort:  "8080",
		ServerAddr:  "127.0.0.1",
		LibraryPath:    "./library",
		UpdateInterval: "",
	}
}

func loadEnv(path string, cfg *Config) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "DB_HOST":
			cfg.DBHost = val
		case "DB_PORT":
			cfg.DBPort = val
		case "DB_USER":
			cfg.DBUser = val
		case "DB_PASSWORD":
			cfg.DBPassword = val
		case "DB_NAME":
			cfg.DBName = val
		case "SERVER_PORT":
			cfg.ServerPort = val
		case "SERVER_ADDRES", "SERVER_ADDRESS":
			cfg.ServerAddr = val
		case "LIBRARY_PATH":
			cfg.LibraryPath = val
		case "DATASTORE_DIR":
			cfg.LibraryPath = val
		case "GITHUB_TOKEN":
			cfg.GithubToken = val
		case "UPDATE_INTERVAL":
			cfg.UpdateInterval = val
		}
	}
	return scanner.Err()
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

func (c *Config) ListenAddr() string {
	return fmt.Sprintf("%s:%s", c.ServerAddr, c.ServerPort)
}
