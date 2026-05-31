package main

import (
	"encoding/json"
)

func getDefaultSettings(userID int64) map[string]interface{} {
	defaults := map[string]interface{}{
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
	
	// Try to get their assigned port
	if db != nil && userID > 0 {
		var port int
		db.QueryRow("SELECT assigned_port FROM users WHERE id = ?", userID).Scan(&port)
		if port > 0 {
			defaults["port"] = port
		}
	}
	
	return defaults
}

func LoadSettings(userID int64) map[string]interface{} {
	defaults := getDefaultSettings(userID)
	if db == nil || userID <= 0 {
		return defaults
	}

	var dataStr string
	err := db.QueryRow("SELECT settings_json FROM user_settings WHERE user_id = ?", userID).Scan(&dataStr)
	if err != nil {
		return defaults
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &parsed); err != nil {
		return defaults
	}

	for k, v := range parsed {
		defaults[k] = v
	}

	return defaults
}

func SaveSettings(userID int64, settings map[string]interface{}) {
	if db == nil || userID <= 0 {
		return
	}
	
	// Auto update port in DB if changed
	if p, ok := settings["port"].(float64); ok {
		db.Exec("UPDATE users SET assigned_port = ? WHERE id = ?", int(p), userID)
	}

	data, err := json.Marshal(settings)
	if err == nil {
		db.Exec(`
			INSERT INTO user_settings (user_id, settings_json) VALUES (?, ?)
			ON CONFLICT(user_id) DO UPDATE SET settings_json=excluded.settings_json
		`, userID, string(data))
	}

	// Hot reload AutoRecord flag in active session manager
	userSessionManagersMu.Lock()
	if sm, exists := userSessionManagers[userID]; exists {
		if autoRecord, ok := settings["autoRecord"].(bool); ok {
			sm.mu.Lock()
			sm.AutoRecord = autoRecord
			sm.mu.Unlock()
		}
	}
	userSessionManagersMu.Unlock()
}
