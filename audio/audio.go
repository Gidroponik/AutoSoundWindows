package audio

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

// GUIDs для Windows Core Audio API
var (
	CLSID_MMDeviceEnumerator = ole.NewGUID("{BCDE0395-E52F-467C-8E3D-C4579291692E}")
	IID_IMMDeviceEnumerator  = ole.NewGUID("{A95664D2-9614-4F35-A746-DE8DB63617E6}")
	IID_IPolicyConfig        = ole.NewGUID("{F8679F50-850A-41CF-9C72-430F290290C8}")
	CLSID_PolicyConfigClient = ole.NewGUID("{870AF99C-171D-4F9E-AF0D-E63DF40C2BC9}")
)

// EDataFlow - направление потока данных
type EDataFlow uint32

const (
	ERender  EDataFlow = 0 // Устройства вывода (колонки, наушники)
	ECapture EDataFlow = 1 // Устройства ввода (микрофоны)
	EAll     EDataFlow = 2
)

// ERole - роль устройства
type ERole uint32

const (
	EConsole       ERole = 0
	EMultimedia    ERole = 1
	ECommunication ERole = 2
)

// DEVICE_STATE константы
const (
	DEVICE_STATE_ACTIVE     = 0x00000001
	DEVICE_STATE_DISABLED   = 0x00000002
	DEVICE_STATE_NOTPRESENT = 0x00000004
	DEVICE_STATE_UNPLUGGED  = 0x00000008
	DEVICE_STATEMASK_ALL    = 0x0000000F
)

// AudioDevice представляет аудиоустройство
type AudioDevice struct {
	ID           string
	Name         string
	IsDefault    bool
	DataFlow     EDataFlow
	FriendlyName string
}

// IMMDeviceEnumerator интерфейс
type IMMDeviceEnumerator struct {
	ole.IUnknown
}

type IMMDeviceEnumeratorVtbl struct {
	ole.IUnknownVtbl
	EnumAudioEndpoints             uintptr
	GetDefaultAudioEndpoint        uintptr
	GetDevice                      uintptr
	RegisterEndpointNotification   uintptr
	UnregisterEndpointNotification uintptr
}

// IMMDeviceCollection интерфейс
type IMMDeviceCollection struct {
	ole.IUnknown
}

type IMMDeviceCollectionVtbl struct {
	ole.IUnknownVtbl
	GetCount uintptr
	Item     uintptr
}

// IMMDevice интерфейс
type IMMDevice struct {
	ole.IUnknown
}

type IMMDeviceVtbl struct {
	ole.IUnknownVtbl
	Activate          uintptr
	OpenPropertyStore uintptr
	GetId             uintptr
	GetState          uintptr
}

// IPropertyStore интерфейс
type IPropertyStore struct {
	ole.IUnknown
}

type IPropertyStoreVtbl struct {
	ole.IUnknownVtbl
	GetCount uintptr
	GetAt    uintptr
	GetValue uintptr
	SetValue uintptr
	Commit   uintptr
}

// PROPERTYKEY структура
type PROPERTYKEY struct {
	Fmtid ole.GUID
	Pid   uint32
}

// PROPVARIANT структура (упрощенная)
type PROPVARIANT struct {
	Vt       uint16
	Reserved [6]byte
	Val      uint64
}

var (
	PKEY_Device_FriendlyName = PROPERTYKEY{
		Fmtid: *ole.NewGUID("{A45C254E-DF1C-4EFD-8020-67D146A850E0}"),
		Pid:   14,
	}
)

// IPolicyConfig интерфейс для установки устройства по умолчанию
type IPolicyConfig struct {
	ole.IUnknown
}

type IPolicyConfigVtbl struct {
	ole.IUnknownVtbl
	GetMixFormat          uintptr
	GetDeviceFormat       uintptr
	ResetDeviceFormat     uintptr
	SetDeviceFormat       uintptr
	GetProcessingPeriod   uintptr
	SetProcessingPeriod   uintptr
	GetShareMode          uintptr
	SetShareMode          uintptr
	GetPropertyValue      uintptr
	SetPropertyValue      uintptr
	SetDefaultEndpoint    uintptr
	SetEndpointVisibility uintptr
}

// AudioManager управляет аудиоустройствами
type AudioManager struct {
	enumerator *IMMDeviceEnumerator
}

var (
	modole32               = syscall.NewLazyDLL("ole32.dll")
	procCoCreateInstance   = modole32.NewProc("CoCreateInstance")
)

const (
	CLSCTX_INPROC_SERVER = 0x1
	CLSCTX_ALL           = 0x17
)

func coCreateInstance(clsid *ole.GUID, iid *ole.GUID, ppv *unsafe.Pointer) error {
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(clsid)),
		0,
		CLSCTX_ALL,
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(ppv)),
	)
	if hr != 0 {
		return fmt.Errorf("CoCreateInstance failed: %x", hr)
	}
	return nil
}

// NewAudioManager создает новый менеджер аудио
func NewAudioManager() (*AudioManager, error) {
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		oleErr, ok := err.(*ole.OleError)
		// S_FALSE (0x00000001) означает, что COM уже инициализирован
		if !ok || (oleErr.Code() != 0 && oleErr.Code() != 1) {
			return nil, fmt.Errorf("failed to initialize COM: %v", err)
		}
	}

	var enumerator *IMMDeviceEnumerator
	err = coCreateInstance(CLSID_MMDeviceEnumerator, IID_IMMDeviceEnumerator, (*unsafe.Pointer)(unsafe.Pointer(&enumerator)))
	if err != nil {
		return nil, err
	}

	return &AudioManager{enumerator: enumerator}, nil
}

// Close освобождает ресурсы
func (am *AudioManager) Close() {
	if am.enumerator != nil {
		am.enumerator.Release()
	}
	ole.CoUninitialize()
}

// GetOutputDevices возвращает список устройств вывода
func (am *AudioManager) GetOutputDevices() ([]AudioDevice, error) {
	return am.getDevices(ERender)
}

// GetInputDevices возвращает список устройств ввода
func (am *AudioManager) GetInputDevices() ([]AudioDevice, error) {
	return am.getDevices(ECapture)
}

func (am *AudioManager) getDevices(dataFlow EDataFlow) ([]AudioDevice, error) {
	vtbl := (*IMMDeviceEnumeratorVtbl)(unsafe.Pointer(am.enumerator.RawVTable))

	var collection *IMMDeviceCollection
	hr, _, _ := syscall.SyscallN(
		vtbl.EnumAudioEndpoints,
		uintptr(unsafe.Pointer(am.enumerator)),
		uintptr(dataFlow),
		uintptr(DEVICE_STATE_ACTIVE),
		uintptr(unsafe.Pointer(&collection)),
	)
	if hr != 0 {
		return nil, fmt.Errorf("failed to enumerate endpoints: %x", hr)
	}
	defer collection.Release()

	// Получаем устройство по умолчанию
	defaultDeviceID := am.getDefaultDeviceID(dataFlow)

	// Получаем количество устройств
	collVtbl := (*IMMDeviceCollectionVtbl)(unsafe.Pointer(collection.RawVTable))
	var count uint32
	hr, _, _ = syscall.SyscallN(
		collVtbl.GetCount,
		uintptr(unsafe.Pointer(collection)),
		uintptr(unsafe.Pointer(&count)),
	)
	if hr != 0 {
		return nil, fmt.Errorf("failed to get device count: %x", hr)
	}

	devices := make([]AudioDevice, 0, count)
	for i := uint32(0); i < count; i++ {
		var device *IMMDevice
		hr, _, _ = syscall.SyscallN(
			collVtbl.Item,
			uintptr(unsafe.Pointer(collection)),
			uintptr(i),
			uintptr(unsafe.Pointer(&device)),
		)
		if hr != 0 {
			continue
		}

		deviceID := am.getDeviceID(device)
		deviceName := am.getDeviceName(device)

		devices = append(devices, AudioDevice{
			ID:           deviceID,
			Name:         deviceName,
			FriendlyName: deviceName,
			IsDefault:    deviceID == defaultDeviceID,
			DataFlow:     dataFlow,
		})

		device.Release()
	}

	return devices, nil
}

func (am *AudioManager) getDefaultDeviceID(dataFlow EDataFlow) string {
	vtbl := (*IMMDeviceEnumeratorVtbl)(unsafe.Pointer(am.enumerator.RawVTable))

	var device *IMMDevice
	hr, _, _ := syscall.SyscallN(
		vtbl.GetDefaultAudioEndpoint,
		uintptr(unsafe.Pointer(am.enumerator)),
		uintptr(dataFlow),
		uintptr(EMultimedia),
		uintptr(unsafe.Pointer(&device)),
	)
	if hr != 0 {
		return ""
	}
	defer device.Release()

	return am.getDeviceID(device)
}

func (am *AudioManager) getDeviceID(device *IMMDevice) string {
	vtbl := (*IMMDeviceVtbl)(unsafe.Pointer(device.RawVTable))

	var pwstrID *uint16
	hr, _, _ := syscall.SyscallN(
		vtbl.GetId,
		uintptr(unsafe.Pointer(device)),
		uintptr(unsafe.Pointer(&pwstrID)),
	)
	if hr != 0 {
		return ""
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(pwstrID)))

	return utf16PtrToString(pwstrID)
}

func (am *AudioManager) getDeviceName(device *IMMDevice) string {
	vtbl := (*IMMDeviceVtbl)(unsafe.Pointer(device.RawVTable))

	var propStore *IPropertyStore
	hr, _, _ := syscall.SyscallN(
		vtbl.OpenPropertyStore,
		uintptr(unsafe.Pointer(device)),
		uintptr(0), // STGM_READ
		uintptr(unsafe.Pointer(&propStore)),
	)
	if hr != 0 {
		return "Unknown Device"
	}
	defer propStore.Release()

	propVtbl := (*IPropertyStoreVtbl)(unsafe.Pointer(propStore.RawVTable))

	var propVar PROPVARIANT
	hr, _, _ = syscall.SyscallN(
		propVtbl.GetValue,
		uintptr(unsafe.Pointer(propStore)),
		uintptr(unsafe.Pointer(&PKEY_Device_FriendlyName)),
		uintptr(unsafe.Pointer(&propVar)),
	)
	if hr != 0 {
		return "Unknown Device"
	}

	if propVar.Vt == 31 { // VT_LPWSTR
		ptr := (*uint16)(unsafe.Pointer(uintptr(propVar.Val)))
		return utf16PtrToString(ptr)
	}

	return "Unknown Device"
}

// SetDefaultDevice устанавливает устройство по умолчанию
func (am *AudioManager) SetDefaultDevice(deviceID string) error {
	var policyConfig *IPolicyConfig

	err := coCreateInstance(CLSID_PolicyConfigClient, IID_IPolicyConfig, (*unsafe.Pointer)(unsafe.Pointer(&policyConfig)))
	if err != nil {
		return err
	}
	defer policyConfig.Release()

	vtbl := (*IPolicyConfigVtbl)(unsafe.Pointer(policyConfig.RawVTable))

	deviceIDPtr, err := syscall.UTF16PtrFromString(deviceID)
	if err != nil {
		return err
	}

	// Устанавливаем для всех ролей
	roles := []ERole{EConsole, EMultimedia, ECommunication}
	for _, role := range roles {
		hr, _, _ := syscall.SyscallN(
			vtbl.SetDefaultEndpoint,
			uintptr(unsafe.Pointer(policyConfig)),
			uintptr(unsafe.Pointer(deviceIDPtr)),
			uintptr(role),
		)
		if hr != 0 {
			return fmt.Errorf("failed to set default endpoint for role %d: %x", role, hr)
		}
	}

	return nil
}

// GetCurrentDefaultOutputID возвращает ID текущего устройства вывода по умолчанию
func (am *AudioManager) GetCurrentDefaultOutputID() string {
	return am.getDefaultDeviceID(ERender)
}

// GetCurrentDefaultInputID возвращает ID текущего устройства ввода по умолчанию
func (am *AudioManager) GetCurrentDefaultInputID() string {
	return am.getDefaultDeviceID(ECapture)
}

func utf16PtrToString(ptr *uint16) string {
	if ptr == nil {
		return ""
	}
	// Находим длину строки
	length := 0
	for p := ptr; *p != 0; p = (*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + 2)) {
		length++
	}
	// Создаем слайс
	slice := unsafe.Slice(ptr, length)
	return syscall.UTF16ToString(slice)
}
