package main

import (
	"context"
	"log"
	"runtime"
	"time"

	"AutoSoundWindows/audio"
	"AutoSoundWindows/settings"

	"github.com/energye/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx             context.Context
	audioManager    *audio.AudioManager
	settingsManager *settings.SettingsManager
	settings        *settings.Settings
	stopNotifier    chan struct{}

	// Временный выбор (до сохранения)
	pendingOutputID string
	pendingInputID  string
}

// AudioDevice для фронтенда
type AudioDeviceInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
	IsChosen  bool   `json:"isChosen"`
	IsPending bool   `json:"isPending"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		stopNotifier: make(chan struct{}),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Инициализация настроек
	var err error
	a.settingsManager, err = settings.NewSettingsManager()
	if err != nil {
		log.Printf("Failed to create settings manager: %v", err)
	}

	a.settings, err = a.settingsManager.Load()
	if err != nil {
		log.Printf("Failed to load settings: %v", err)
		a.settings = &settings.Settings{AutoSwitch: true}
	}

	// Инициализируем pending значения из сохранённых
	a.pendingOutputID = a.settings.OutputDeviceID
	a.pendingInputID = a.settings.InputDeviceID

	// Инициализация аудио менеджера
	a.audioManager, err = audio.NewAudioManager()
	if err != nil {
		log.Printf("Failed to create audio manager: %v", err)
	}

	// Запускаем systray
	go a.initSystray()

	// Запускаем отслеживание изменений
	go a.startDeviceNotifier()
}

// shutdown is called when the app closes
func (a *App) shutdown(ctx context.Context) {
	close(a.stopNotifier)
	systray.Quit()
	if a.audioManager != nil {
		a.audioManager.Close()
	}
}

func (a *App) initSystray() {
	systray.Run(func() {
		// Используем встроенную иконку Windows (динамик)
		systray.SetIcon(icon)
		systray.SetTitle("AutoSound")
		systray.SetTooltip("AutoSound - Управление аудиоустройствами")

		mShow := systray.AddMenuItem("Открыть", "Открыть окно")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Выход", "Закрыть программу")

		// Клик по иконке
		systray.SetOnClick(func(menu systray.IMenu) {
			wailsRuntime.WindowShow(a.ctx)
		})

		systray.SetOnDClick(func(menu systray.IMenu) {
			wailsRuntime.WindowShow(a.ctx)
		})

		mShow.Click(func() {
			wailsRuntime.WindowShow(a.ctx)
		})

		mQuit.Click(func() {
			wailsRuntime.Quit(a.ctx)
		})
	}, nil)
}

// ShowWindow показывает окно
func (a *App) ShowWindow() {
	wailsRuntime.WindowShow(a.ctx)
}

// MinimizeWindow сворачивает окно
func (a *App) MinimizeWindow() {
	wailsRuntime.WindowMinimise(a.ctx)
}

// HideWindow скрывает окно в трей
func (a *App) HideWindow() {
	wailsRuntime.WindowHide(a.ctx)
}

// GetOutputDevices возвращает устройства вывода
func (a *App) GetOutputDevices() []AudioDeviceInfo {
	if a.audioManager == nil {
		return []AudioDeviceInfo{}
	}

	devices, err := a.audioManager.GetOutputDevices()
	if err != nil {
		log.Printf("Failed to get output devices: %v", err)
		return []AudioDeviceInfo{}
	}

	result := make([]AudioDeviceInfo, len(devices))
	for i, dev := range devices {
		result[i] = AudioDeviceInfo{
			ID:        dev.ID,
			Name:      dev.Name,
			IsDefault: dev.IsDefault,
			IsChosen:  dev.ID == a.settings.OutputDeviceID,
			IsPending: dev.ID == a.pendingOutputID,
		}
	}
	return result
}

// GetInputDevices возвращает устройства ввода
func (a *App) GetInputDevices() []AudioDeviceInfo {
	if a.audioManager == nil {
		return []AudioDeviceInfo{}
	}

	devices, err := a.audioManager.GetInputDevices()
	if err != nil {
		log.Printf("Failed to get input devices: %v", err)
		return []AudioDeviceInfo{}
	}

	result := make([]AudioDeviceInfo, len(devices))
	for i, dev := range devices {
		result[i] = AudioDeviceInfo{
			ID:        dev.ID,
			Name:      dev.Name,
			IsDefault: dev.IsDefault,
			IsChosen:  dev.ID == a.settings.InputDeviceID,
			IsPending: dev.ID == a.pendingInputID,
		}
	}
	return result
}

// SelectOutputDevice выбирает устройство (временно, до сохранения)
func (a *App) SelectOutputDevice(deviceID string) {
	a.pendingOutputID = deviceID
}

// SelectInputDevice выбирает устройство (временно, до сохранения)
func (a *App) SelectInputDevice(deviceID string) {
	a.pendingInputID = deviceID
}

// SaveSettings сохраняет выбранные устройства и применяет их
func (a *App) SaveSettings() error {
	if a.audioManager == nil {
		return nil
	}

	// Применяем устройство вывода
	if a.pendingOutputID != "" && a.pendingOutputID != a.settings.OutputDeviceID {
		if err := a.audioManager.SetDefaultDevice(a.pendingOutputID); err != nil {
			log.Printf("Failed to set output device: %v", err)
			return err
		}
		a.settings.OutputDeviceID = a.pendingOutputID
	}

	// Применяем устройство ввода
	if a.pendingInputID != "" && a.pendingInputID != a.settings.InputDeviceID {
		if err := a.audioManager.SetDefaultDevice(a.pendingInputID); err != nil {
			log.Printf("Failed to set input device: %v", err)
			return err
		}
		a.settings.InputDeviceID = a.pendingInputID
	}

	// Сохраняем настройки
	a.settingsManager.Save(a.settings)
	log.Printf("Settings saved: output=%s, input=%s", a.settings.OutputDeviceID, a.settings.InputDeviceID)
	return nil
}

// HasUnsavedChanges проверяет есть ли несохранённые изменения
func (a *App) HasUnsavedChanges() bool {
	return a.pendingOutputID != a.settings.OutputDeviceID ||
		a.pendingInputID != a.settings.InputDeviceID
}

// ResetChanges сбрасывает несохранённые изменения
func (a *App) ResetChanges() {
	a.pendingOutputID = a.settings.OutputDeviceID
	a.pendingInputID = a.settings.InputDeviceID
}

// GetAutoSwitch возвращает состояние автопереключения
func (a *App) GetAutoSwitch() bool {
	return a.settings.AutoSwitch
}

// SetAutoSwitch устанавливает автопереключение
func (a *App) SetAutoSwitch(enabled bool) {
	a.settings.AutoSwitch = enabled
	a.settingsManager.Save(a.settings)
}

// Quit закрывает приложение
func (a *App) Quit() {
	wailsRuntime.Quit(a.ctx)
}

// GetAutostartEnabled возвращает состояние автозапуска
func (a *App) GetAutostartEnabled() bool {
	return settings.IsAutostartEnabled()
}

// SetAutostartEnabled устанавливает автозапуск
func (a *App) SetAutostartEnabled(enabled bool) error {
	return settings.SetAutostart(enabled)
}

// ShouldShowAutostartPrompt проверяет нужно ли показать запрос на автозапуск
func (a *App) ShouldShowAutostartPrompt() bool {
	// Показываем только если ещё не спрашивали и автозапуск не включен
	return !a.settings.AutostartAsked && !settings.IsAutostartEnabled()
}

// MarkAutostartAsked помечает что пользователя спросили об автозапуске
func (a *App) MarkAutostartAsked() {
	a.settings.AutostartAsked = true
	a.settingsManager.Save(a.settings)
}

func (a *App) startDeviceNotifier() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	audioMgr, err := audio.NewAudioManager()
	if err != nil {
		log.Printf("Failed to create audio manager for notifier: %v", err)
		return
	}
	defer audioMgr.Close()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopNotifier:
			return
		case <-ticker.C:
			if !a.settings.AutoSwitch {
				continue
			}

			// Проверяем устройство вывода
			currentOutputID := audioMgr.GetCurrentDefaultOutputID()
			if a.settings.OutputDeviceID != "" && currentOutputID != a.settings.OutputDeviceID {
				log.Printf("Output device changed externally, restoring...")
				audioMgr.SetDefaultDevice(a.settings.OutputDeviceID)
			}

			// Проверяем устройство ввода
			currentInputID := audioMgr.GetCurrentDefaultInputID()
			if a.settings.InputDeviceID != "" && currentInputID != a.settings.InputDeviceID {
				log.Printf("Input device changed externally, restoring...")
				audioMgr.SetDefaultDevice(a.settings.InputDeviceID)
			}
		}
	}
}
