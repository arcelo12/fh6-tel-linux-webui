package main

import (
	"encoding/json"
	"log"
	"os"
)

type SmtpConfig struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
}

type AuthConfig struct {
	MaxLoginAttempts    int `json:"maxLoginAttempts"`    // per 15 minutes
	MaxRegisterAttempts int `json:"maxRegisterAttempts"` // per hour
}

type AppConfig struct {
	Smtp SmtpConfig `json:"smtp"`
	Auth AuthConfig `json:"auth"`
}

var globalConfig AppConfig

// loadConfig reads fh6_config.json from the current directory.
// If it doesn't exist, it creates a default configuration file.
func loadConfig() {
	// Set defaults
	globalConfig = AppConfig{
		Smtp: SmtpConfig{
			Enabled:  false,
			Host:     "smtp.example.com",
			Port:     587,
			Username: "your_email@example.com",
			Password: "your_password",
			From:     "noreply@example.com",
		},
		Auth: AuthConfig{
			MaxLoginAttempts:    10,
			MaxRegisterAttempts: 5,
		},
	}

	configFileName := "fh6_config.json"
	data, err := os.ReadFile(configFileName)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config file
			defaultData, _ := json.MarshalIndent(globalConfig, "", "  ")
			err = os.WriteFile(configFileName, defaultData, 0644)
			if err != nil {
				log.Printf("[Config] Warning: Failed to create default config file: %v", err)
			} else {
				log.Printf("[Config] Created default configuration file: %s", configFileName)
			}
		} else {
			log.Printf("[Config] Warning: Failed to read %s: %v", configFileName, err)
		}
		return
	}

	// Parse existing config
	err = json.Unmarshal(data, &globalConfig)
	if err != nil {
		log.Printf("[Config] Error parsing %s, using defaults. Error: %v", configFileName, err)
		return
	}

	log.Printf("[Config] Successfully loaded configuration from %s", configFileName)
}
