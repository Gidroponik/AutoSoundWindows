package main

import (
	"log"
	"runtime"
	"time"

	"AutoSoundWindows/audio"
	"AutoSoundWindows/settings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

type AppWindow struct {
	*walk.MainWindow
	outputList      *walk.ListBox
	inputList       *walk.ListBox
	autoSwitchCheck *walk.CheckBox
	ni              *walk.NotifyIcon

	audioManager    *audio.AudioManager
	settingsManager *settings.SettingsManager

	outputDevices []audio.AudioDevice
	inputDevices  []audio.AudioDevice

	currentSettings *settings.Settings
	isRefreshing    bool
}

func main() {
	app := &AppWindow{}

	// Инициализация менеджера настроек
	var err error
	app.settingsManager, err = settings.NewSettingsManager()
	if err != nil {
		log.Fatalf("Failed to create settings manager: %v", err)
	}

	// Загрузка настроек
	app.currentSettings, err = app.settingsManager.Load()
	if err != nil {
		log.Printf("Failed to load settings: %v", err)
		app.currentSettings = &settings.Settings{AutoSwitch: true}
	}

	// Инициализация аудио менеджера
	app.audioManager, err = audio.NewAudioManager()
	if err != nil {
		log.Fatalf("Failed to create audio manager: %v", err)
	}
	defer app.audioManager.Close()

	// Создаем иконку для трея
	icon, _ := walk.NewIconFromSysDLL("shell32", 138)

	// Создаем главное окно
	if err := (MainWindow{
		AssignTo: &app.MainWindow,
		Title:    "AutoSound",
		MinSize:  Size{Width: 600, Height: 400},
		Size:     Size{Width: 700, Height: 450},
		Layout:   VBox{},
		Visible:  false,
		Children: []Widget{
			Composite{
				Layout: HBox{},
				Children: []Widget{
					GroupBox{
						Title:  "Устройства вывода (колонки/наушники)",
						Layout: VBox{},
						Children: []Widget{
							ListBox{
								AssignTo:       &app.outputList,
								MultiSelection: false,
								OnItemActivated: func() {
									app.onOutputSelected()
								},
							},
							PushButton{
								Text: "Выбрать устройство вывода",
								OnClicked: func() {
									app.onOutputSelected()
								},
							},
						},
					},
					GroupBox{
						Title:  "Устройства ввода (микрофоны)",
						Layout: VBox{},
						Children: []Widget{
							ListBox{
								AssignTo:       &app.inputList,
								MultiSelection: false,
								OnItemActivated: func() {
									app.onInputSelected()
								},
							},
							PushButton{
								Text: "Выбрать устройство ввода",
								OnClicked: func() {
									app.onInputSelected()
								},
							},
						},
					},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					CheckBox{
						AssignTo: &app.autoSwitchCheck,
						Text:     "Автоматически возвращать выбранное устройство",
						Checked:  app.currentSettings.AutoSwitch,
						OnCheckedChanged: func() {
							app.currentSettings.AutoSwitch = app.autoSwitchCheck.Checked()
							app.saveSettings()
						},
					},
					HSpacer{},
					PushButton{
						Text: "Обновить список",
						OnClicked: func() {
							app.refreshDevices()
						},
					},
				},
			},
		},
	}).Create(); err != nil {
		log.Fatal(err)
	}

	// Устанавливаем иконку окна
	if icon != nil {
		app.SetIcon(icon)
	}

	// Создаем NotifyIcon
	app.ni, err = walk.NewNotifyIcon(app.MainWindow)
	if err != nil {
		log.Fatal(err)
	}
	defer app.ni.Dispose()

	if icon != nil {
		app.ni.SetIcon(icon)
	}
	app.ni.SetToolTip("AutoSound - Управление аудиоустройствами")
	app.ni.SetVisible(true)

	// Обработка закрытия окна
	app.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		*canceled = true
		app.Hide()
	})

	// Клик по иконке в трее
	app.ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			if app.Visible() {
				app.Hide()
			} else {
				app.Show()
				app.Activate()
			}
		}
	})

	// Контекстное меню
	showAction := walk.NewAction()
	showAction.SetText("Показать")
	showAction.Triggered().Attach(func() {
		app.Show()
		app.Activate()
	})

	refreshAction := walk.NewAction()
	refreshAction.SetText("Обновить устройства")
	refreshAction.Triggered().Attach(func() {
		app.refreshDevices()
	})

	exitAction := walk.NewAction()
	exitAction.SetText("Выход")
	exitAction.Triggered().Attach(func() {
		app.ni.Dispose()
		app.Dispose()
		walk.App().Exit(0)
	})

	app.ni.ContextMenu().Actions().Add(showAction)
	app.ni.ContextMenu().Actions().Add(refreshAction)
	app.ni.ContextMenu().Actions().Add(walk.NewSeparatorAction())
	app.ni.ContextMenu().Actions().Add(exitAction)

	// Загружаем устройства
	app.refreshDevices()

	// Запускаем отслеживание изменений
	app.startDeviceNotifier()

	// Запускаем в свёрнутом виде (в трее)
	app.Run()
}

func (app *AppWindow) refreshDevices() {
	app.isRefreshing = true
	defer func() { app.isRefreshing = false }()

	// Получаем устройства
	var err error
	app.outputDevices, err = app.audioManager.GetOutputDevices()
	if err != nil {
		log.Printf("Failed to get output devices: %v", err)
	}

	app.inputDevices, err = app.audioManager.GetInputDevices()
	if err != nil {
		log.Printf("Failed to get input devices: %v", err)
	}

	// Обновляем список вывода
	outputItems := make([]string, len(app.outputDevices))
	selectedOutput := -1
	for i, dev := range app.outputDevices {
		marker := "   "
		if dev.ID == app.currentSettings.OutputDeviceID {
			marker = ">> "
			selectedOutput = i
		} else if dev.IsDefault {
			marker = " * "
		}
		outputItems[i] = marker + dev.Name
	}

	if err := app.outputList.SetModel(outputItems); err != nil {
		log.Printf("Failed to set output model: %v", err)
	}
	if selectedOutput >= 0 {
		app.outputList.SetCurrentIndex(selectedOutput)
	}

	// Обновляем список ввода
	inputItems := make([]string, len(app.inputDevices))
	selectedInput := -1
	for i, dev := range app.inputDevices {
		marker := "   "
		if dev.ID == app.currentSettings.InputDeviceID {
			marker = ">> "
			selectedInput = i
		} else if dev.IsDefault {
			marker = " * "
		}
		inputItems[i] = marker + dev.Name
	}

	if err := app.inputList.SetModel(inputItems); err != nil {
		log.Printf("Failed to set input model: %v", err)
	}
	if selectedInput >= 0 {
		app.inputList.SetCurrentIndex(selectedInput)
	}
}

func (app *AppWindow) onOutputSelected() {
	idx := app.outputList.CurrentIndex()
	if idx < 0 || idx >= len(app.outputDevices) {
		return
	}

	device := app.outputDevices[idx]
	log.Printf("Selecting output device: %s", device.Name)

	// Устанавливаем устройство по умолчанию
	if err := app.audioManager.SetDefaultDevice(device.ID); err != nil {
		log.Printf("Failed to set output device: %v", err)
		walk.MsgBox(app, "Ошибка", "Не удалось установить устройство: "+err.Error(), walk.MsgBoxIconError)
		return
	}

	app.currentSettings.OutputDeviceID = device.ID
	app.saveSettings()
	app.refreshDevices()
}

func (app *AppWindow) onInputSelected() {
	idx := app.inputList.CurrentIndex()
	if idx < 0 || idx >= len(app.inputDevices) {
		return
	}

	device := app.inputDevices[idx]
	log.Printf("Selecting input device: %s", device.Name)

	// Устанавливаем устройство по умолчанию
	if err := app.audioManager.SetDefaultDevice(device.ID); err != nil {
		log.Printf("Failed to set input device: %v", err)
		walk.MsgBox(app, "Ошибка", "Не удалось установить устройство: "+err.Error(), walk.MsgBoxIconError)
		return
	}

	app.currentSettings.InputDeviceID = device.ID
	app.saveSettings()
	app.refreshDevices()
}

func (app *AppWindow) saveSettings() {
	if err := app.settingsManager.Save(app.currentSettings); err != nil {
		log.Printf("Failed to save settings: %v", err)
	}
}

func (app *AppWindow) startDeviceNotifier() {
	go func() {
		// Привязываем горутину к одному потоку ОС для COM
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		// Инициализируем COM в этой горутине
		audioMgr, err := audio.NewAudioManager()
		if err != nil {
			log.Printf("Failed to create audio manager for notifier: %v", err)
			return
		}
		defer audioMgr.Close()

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if !app.currentSettings.AutoSwitch {
				continue
			}

			// Проверяем устройство вывода
			currentOutputID := audioMgr.GetCurrentDefaultOutputID()
			if app.currentSettings.OutputDeviceID != "" && currentOutputID != app.currentSettings.OutputDeviceID {
				log.Printf("Output device changed externally, restoring...")
				if err := audioMgr.SetDefaultDevice(app.currentSettings.OutputDeviceID); err != nil {
					log.Printf("Failed to restore output device: %v", err)
				}
			}

			// Проверяем устройство ввода
			currentInputID := audioMgr.GetCurrentDefaultInputID()
			if app.currentSettings.InputDeviceID != "" && currentInputID != app.currentSettings.InputDeviceID {
				log.Printf("Input device changed externally, restoring...")
				if err := audioMgr.SetDefaultDevice(app.currentSettings.InputDeviceID); err != nil {
					log.Printf("Failed to restore input device: %v", err)
				}
			}
		}
	}()
}
