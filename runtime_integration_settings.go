package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

const (
	runtimeRemoteDBPasswordSettingKey = "runtime.remote_db_password"
	runtimeOCRServiceTokenSettingKey  = "runtime.ocr_service_token"
)

func isSecretPlaceholder(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}
	if trimmed == "(not set)" || trimmed == "(not configured)" {
		return true
	}
	return strings.Contains(trimmed, "*")
}

func cloneSettingsMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return map[string]interface{}{}
	}

	data, err := json.Marshal(input)
	if err != nil {
		log.Printf("Warning: failed to clone settings map during save: %v", err)
		return map[string]interface{}{}
	}

	var cloned map[string]interface{}
	if err := json.Unmarshal(data, &cloned); err != nil {
		log.Printf("Warning: failed to decode cloned settings map during save: %v", err)
		return map[string]interface{}{}
	}

	return cloned
}

func getStringSetting(settings map[string]interface{}, key, defaultValue string) string {
	value := getSettingOrDefault(settings, key, defaultValue)
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func getBoolSetting(settings map[string]interface{}, key string, defaultValue bool) bool {
	value := getSettingOrDefault(settings, key, defaultValue)
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err == nil {
			return parsed
		}
	case float64:
		return v != 0
	case int:
		return v != 0
	}
	return defaultValue
}

func setOrUnsetRuntimeEnv(name, value string) error {
	if strings.TrimSpace(value) == "" {
		if err := os.Unsetenv(name); err != nil {
			return fmt.Errorf("failed to clear runtime setting %s: %w", name, err)
		}
		return nil
	}
	return setRuntimeEnv(name, value)
}

func (a *App) getStoredSettingValue(settingKey, fallbackJSONKey string, userSettings map[string]interface{}) string {
	if a.settingsService != nil {
		if value, err := a.settingsService.GetSetting(settingKey); err == nil && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return getStringSetting(userSettings, fallbackJSONKey, "")
}

func (a *App) getStoredSecret(settingKey, fallbackJSONKey string, userSettings map[string]interface{}) string {
	if a.settingsService != nil {
		if value, err := a.settingsService.GetSetting(settingKey); err == nil && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}

	fallback := getStringSetting(userSettings, fallbackJSONKey, "")
	if isSecretPlaceholder(fallback) {
		return ""
	}
	return fallback
}

func (a *App) getMaskedStoredSecret(settingKey, fallbackJSONKey string, userSettings map[string]interface{}) string {
	return maskSecret(a.getStoredSecret(settingKey, fallbackJSONKey, userSettings))
}

func (a *App) persistSecretSetting(settingKey, value, category string) error {
	trimmed := strings.TrimSpace(value)
	if isSecretPlaceholder(trimmed) {
		return nil
	}
	if a.settingsService == nil {
		return nil
	}
	return a.settingsService.SetSetting(settingKey, trimmed, category, true)
}

func (a *App) applyPersistedRuntimeSettings() error {
	userSettings, err := a.loadUserSettings()
	if err != nil {
		return fmt.Errorf("load persisted runtime settings: %w", err)
	}

	syncEnabled := getBoolSetting(userSettings, "sync.enabled", false)
	if err := setOrUnsetRuntimeEnv("ASYMMFLOW_REMOTE_SYNC_ENABLED", strconv.FormatBool(syncEnabled)); err != nil {
		return err
	}
	if err := setOrUnsetRuntimeEnv("ASYMMFLOW_REMOTE_DB_HOST", getStringSetting(userSettings, "sync.db_host", "")); err != nil {
		return err
	}
	if err := setOrUnsetRuntimeEnv("ASYMMFLOW_REMOTE_DB_PORT", getStringSetting(userSettings, "sync.db_port", "5432")); err != nil {
		return err
	}
	if err := setOrUnsetRuntimeEnv("ASYMMFLOW_REMOTE_DB_NAME", getStringSetting(userSettings, "sync.db_name", "postgres")); err != nil {
		return err
	}
	if err := setOrUnsetRuntimeEnv("ASYMMFLOW_REMOTE_DB_USER", getStringSetting(userSettings, "sync.db_user", "postgres")); err != nil {
		return err
	}
	if err := setOrUnsetRuntimeEnv("ASYMMFLOW_REMOTE_DB_SSLMODE", getStringSetting(userSettings, "sync.db_sslmode", "require")); err != nil {
		return err
	}
	if err := setOrUnsetRuntimeEnv("ASYMMFLOW_REMOTE_DB_PASSWORD", a.getStoredSecret(runtimeRemoteDBPasswordSettingKey, "sync.db_password", userSettings)); err != nil {
		return err
	}

	if err := setOrUnsetRuntimeEnv("ASYMMFLOW_OCR_ENGINE_URL", getStringSetting(userSettings, "ocr.engine_url", "")); err != nil {
		return err
	}
	if err := setOrUnsetRuntimeEnv("ASYMMFLOW_OCR_SERVICE_TOKEN", a.getStoredSecret(runtimeOCRServiceTokenSettingKey, "ocr.service_token", userSettings)); err != nil {
		return err
	}

	return nil
}

func (a *App) refreshRuntimeIntegrations() error {
	if err := a.applyPersistedRuntimeSettings(); err != nil {
		return err
	}

	if ocrSvc, err := NewSimpleOCRService(); err != nil {
		log.Printf("Warning: failed to refresh OCR runtime settings: %v", err)
	} else {
		a.ocrService = ocrSvc
	}

	if a.dbManager != nil {
		a.dbManager.StopPeriodicSync()
		a.dbManager.Disconnect()
	}

	a.InitDBManager()
	return nil
}
