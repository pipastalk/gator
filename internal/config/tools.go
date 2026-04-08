package config

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DB_URL          string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

const configFileName = "gatorconfig.json"

func getConfigFilePath() (string, error) {
	if configFileName == "" {
		return "", errors.New("config file path is not set")
	}
	folder, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configFilePath := filepath.Join(folder, configFileName)
	return configFilePath, nil
}

func (cfg *Config) write() error {
	if err := cfg.verify(); err != nil {
		return err
	}
	configPath, err := getConfigFilePath()
	if err != nil {
		return err
	}
	jsonData, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	err = os.WriteFile(configPath, jsonData, 0644)
	return err //expected to be nil if WriteFile works
}

func Read() (*Config, error) {
	var cfg Config
	configPath, err := getConfigFilePath()
	if err != nil {
		return nil, err
	}
	jsonData, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		// Create a default config if file doesn't exist
		defaultCfg := Config{
			DB_URL:          "",
			CurrentUserName: "",
		}
		cfg = defaultCfg

	} else {
		err = json.Unmarshal(jsonData, &cfg)
		if err != nil {
			return nil, fmt.Errorf("Failed to unmarshal config data:\n config_path %v,\nerror: %w", configPath, err)
		}
	}
	return &cfg, nil
}

func SetUsername(username string) error {
	cfg, err := Read()
	if err != nil {
		return fmt.Errorf("Error reading config: %w", err)
	}
	cfg.CurrentUserName = username
	if err := cfg.write(); err != nil {
		return fmt.Errorf("Error setting config: %w", err)
	}
	return nil
}

func (cfg *Config) verify() error {
	if cfg.DB_URL == "" {
		return errors.New("DB_URL is required in config")
	}
	if cfg.CurrentUserName == "" {
		return errors.New("CurrentUserName is required in config")
	}
	return nil
}
