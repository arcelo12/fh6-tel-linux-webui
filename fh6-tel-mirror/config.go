package main

import (
	"encoding/json"
	"os"
	"sync"
)

type Destination struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	RateLimit string `json:"rateLimit"` // "60Hz", "30Hz", "20Hz", "10Hz"
	Enabled   bool   `json:"enabled"`
}

type Config struct {
	BindPort     int           `json:"bindPort"`     // Inbound UDP port (e.g. 20440)
	Destinations []Destination `json:"destinations"`
}

var (
	config     Config
	configLock sync.RWMutex
	configFile = "mirror_config.json"
)

func loadConfig() error {
	configLock.Lock()
	defer configLock.Unlock()

	// Default config setup
	config = Config{
		BindPort:    20440,
		Destinations: []Destination{
			{
				ID:        "simhub",
				Name:      "SimHub/Vibration Rig",
				Host:      "127.0.0.1",
				Port:      20500,
				RateLimit: "60Hz",
				Enabled:   false,
			},
			{
				ID:        "local-hub",
				Name:      "Local FH6 Telemetry Server",
				Host:      "127.0.0.1",
				Port:      20450, // Alternate port
				RateLimit: "60Hz",
				Enabled:   true,
			},
		},
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Save default setup
			return saveConfigLocked()
		}
		return err
	}

	return json.Unmarshal(data, &config)
}

func saveConfig() error {
	configLock.Lock()
	defer configLock.Unlock()
	return saveConfigLocked()
}

func saveConfigLocked() error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
}
