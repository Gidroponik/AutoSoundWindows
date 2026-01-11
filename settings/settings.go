package settings

import (
	"encoding/json"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	registryKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	registryValue = "AutoSound"
)

// Settings хранит настройки приложения
type Settings struct {
	OutputDeviceID string `json:"output_device_id"`
	InputDeviceID  string `json:"input_device_id"`
	AutoSwitch     bool   `json:"auto_switch"`
	AutostartAsked bool   `json:"autostart_asked"`
}

// SettingsManager управляет настройками
type SettingsManager struct {
	filePath string
	settings *Settings
}

// NewSettingsManager создает новый менеджер настроек
func NewSettingsManager() (*SettingsManager, error) {
	appData, err := os.UserConfigDir()
	if err != nil {
		appData = os.Getenv("APPDATA")
		if appData == "" {
			appData = "."
		}
	}

	settingsDir := filepath.Join(appData, "AutoSound")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return nil, err
	}

	return &SettingsManager{
		filePath: filepath.Join(settingsDir, "settings.json"),
		settings: &Settings{AutoSwitch: true},
	}, nil
}

// Load загружает настройки из файла
func (sm *SettingsManager) Load() (*Settings, error) {
	data, err := os.ReadFile(sm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Файл не существует, возвращаем настройки по умолчанию
			return sm.settings, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, sm.settings); err != nil {
		return nil, err
	}

	return sm.settings, nil
}

// Save сохраняет настройки в файл
func (sm *SettingsManager) Save(settings *Settings) error {
	sm.settings = settings

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sm.filePath, data, 0644)
}

// GetSettings возвращает текущие настройки
func (sm *SettingsManager) GetSettings() *Settings {
	return sm.settings
}

// GetFilePath возвращает путь к файлу настроек
func (sm *SettingsManager) GetFilePath() string {
	return sm.filePath
}

// IsAutostartEnabled проверяет включен ли автозапуск в реестре
func IsAutostartEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	_, _, err = key.GetStringValue(registryValue)
	return err == nil
}

// SetAutostart включает или выключает автозапуск
func SetAutostart(enabled bool) error {
	if enabled {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}

		key, err := registry.OpenKey(registry.CURRENT_USER, registryKey, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer key.Close()

		return key.SetStringValue(registryValue, `"`+exePath+`"`)
	} else {
		key, err := registry.OpenKey(registry.CURRENT_USER, registryKey, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer key.Close()

		return key.DeleteValue(registryValue)
	}
}
