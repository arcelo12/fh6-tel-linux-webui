package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func settingsPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".local", "share", "fh6-tel", "settings.json")
}

func getDefaultSettings() map[string]interface{} {
	return map[string]interface{}{
		"port": 20440,
		"useMph": true,
		"tireTempCold": 60.0,
		"tireTempOptimal": 85.0,
		"tireTempHot": 110.0,
		"autoRecord": true,
		"theme": "dark",
		"mapEnabled": false,
		"mapOverride": false,
		"mapTileUrl": "",
		"mapMinZoom": 0,
		"mapMaxZoom": 5,
		"mapTileSize": 256,
		"mapCalAWorld": []float64{0.0, 0.0},
		"mapCalAPix": []float64{0.0, 0.0},
		"mapCalBWorld": []float64{0.0, 0.0},
		"mapCalBPix": []float64{0.0, 0.0},
		"mapViewMaxZoom": 0,
		"mapDefaultZoom": 0,
		"mapDefaultCenter": []float64{0.0, 0.0},
	}
}

func LoadSettings() map[string]interface{} {
	defaults := getDefaultSettings()
	p := settingsPath()
	
	data, err := os.ReadFile(p)
	if err != nil {
		return defaults
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return defaults
	}

	// Merge with defaults
	for k, v := range parsed {
		defaults[k] = v
	}

	return defaults
}

func SaveSettings(settings map[string]interface{}) {
	p := settingsPath()
	os.MkdirAll(filepath.Dir(p), 0755)
	
	data, err := json.MarshalIndent(settings, "", "  ")
	if err == nil {
		os.WriteFile(p, data, 0644)
	}
}
