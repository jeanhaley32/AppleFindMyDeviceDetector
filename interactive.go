package main

import (
	"bufio"
	"os"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

// Mode represents the current operating mode
type Mode string

const (
	ModeScan    Mode = "scan"
	ModeConnect Mode = "connect"
)

// InteractiveController manages interactive sound control for detected AirTags
type InteractiveController struct {
	adapter      *bluetooth.Adapter
	soundTrigger *SoundTrigger
	devices      *sync.Map // address string -> device info
	soundPlaying bool      // toggle state
	mode         Mode      // current mode: scan or connect
	mu           sync.Mutex
	onStopScan   func() // callback to stop scanning
	onStartScan  func() // callback to resume scanning
}

// DeviceInfo tracks a detected device and its connection history
type DeviceInfo struct {
	Address       bluetooth.Address
	LastSeen      time.Time
	ConnectStatus string // "pending", "queued", "testing", "ok", "fail", etc.
	LastError     string
	Connection    *bluetooth.Device             // Active connection (if any)
	Characteristic *bluetooth.DeviceCharacteristic // Sound control characteristic
}

// NewInteractiveController creates a new interactive controller
func NewInteractiveController(adapter *bluetooth.Adapter) *InteractiveController {
	return &InteractiveController{
		adapter:      adapter,
		soundTrigger: NewSoundTrigger(adapter),
		devices:      new(sync.Map),
		mode:         ModeScan,
	}
}

// SetScanCallbacks sets the callbacks for controlling scanning
func (ic *InteractiveController) SetScanCallbacks(onStop, onStart func()) {
	ic.onStopScan = onStop
	ic.onStartScan = onStart
}

// GetMode returns the current mode
func (ic *InteractiveController) GetMode() Mode {
	return ic.mode
}

// RegisterDevice adds or updates a device in our tracking list
// Marks new devices as "pending" - connection test happens when Enter is pressed
func (ic *InteractiveController) RegisterDevice(address bluetooth.Address) {
	addrStr := address.String()

	if existing, ok := ic.devices.Load(addrStr); ok {
		info := existing.(*DeviceInfo)
		info.LastSeen = time.Now()
		return
	}

	// New device - mark as pending (will test when user presses Enter)
	info := &DeviceInfo{
		Address:       address,
		LastSeen:      time.Now(),
		ConnectStatus: "pending",
	}
	ic.devices.Store(addrStr, info)
}

// GetDeviceCount returns the number of tracked devices
func (ic *InteractiveController) GetDeviceCount() int {
	count := 0
	ic.devices.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// testConnection attempts to connect to a device and maintain the connection
func (ic *InteractiveController) testConnection(addrStr string, info *DeviceInfo) {
	// Try to connect
	device, err := ic.adapter.Connect(info.Address, bluetooth.ConnectionParams{
		ConnectionTimeout: bluetooth.NewDuration(10 * time.Second),
	})
	if err != nil {
		info.ConnectStatus = "no conn"
		info.LastError = err.Error()
		return
	}

	// Store the connection (don't disconnect!)
	info.Connection = device

	// Discover ALL services to see what's available
	allServices, err := device.DiscoverServices(nil)
	if err != nil {
		info.ConnectStatus = "no svc"
		info.LastError = err.Error()
		return
	}

	if len(allServices) == 0 {
		info.ConnectStatus = "no svc"
		info.LastError = "none found"
		return
	}

	// Log discovered services for debugging
	var serviceUUIDs []string
	for _, svc := range allServices {
		serviceUUIDs = append(serviceUUIDs, svc.UUID().String())
	}

	// Store what we found for display
	info.ConnectStatus = "conn"
	info.LastError = serviceUUIDs[0] // Show first service UUID found

	// Check if our expected service is there
	for _, svc := range allServices {
		if svc.UUID() == NonOwnerServiceUUID {
			// Found the non-owner service, try to get characteristic
			chars, err := svc.DiscoverCharacteristics(nil)
			if err == nil && len(chars) > 0 {
				info.Characteristic = &chars[0]
				info.ConnectStatus = "ready"
				info.LastError = ""
				return
			}
		}
	}
}

// GetDeviceStatus returns the connection status for a device address
func (ic *InteractiveController) GetDeviceStatus(addrStr string) string {
	if info, ok := ic.devices.Load(addrStr); ok {
		return info.(*DeviceInfo).ConnectStatus
	}
	return ""
}

// GetConnectionStats returns stats about connection attempts (total, ok, testing, failed)
func (ic *InteractiveController) GetConnectionStats() (total, ok, testing, failed int) {
	ic.devices.Range(func(_, value interface{}) bool {
		total++
		info := value.(*DeviceInfo)
		switch info.ConnectStatus {
		case "ready", "conn":
			ok++
		case "testing", "queued":
			testing++
		case "pending":
			// Not counted as failed yet
		default:
			// All other statuses (no conn, no svc, etc.) are failures
			if info.ConnectStatus != "" {
				failed++
			}
		}
		return true
	})
	return
}

// TriggerAllSounds sends play sound command to all devices with active connections
func (ic *InteractiveController) TriggerAllSounds() {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	ic.devices.Range(func(key, value interface{}) bool {
		info := value.(*DeviceInfo)

		// Only try devices that have the characteristic ready
		if info.ConnectStatus != "ready" || info.Characteristic == nil {
			return true
		}

		// Send sound command using existing connection
		cmd := make([]byte, 2)
		cmd[0] = byte(OpcodeSoundStart & 0xFF)
		cmd[1] = byte(OpcodeSoundStart >> 8)

		info.Characteristic.WriteWithoutResponse(cmd)
		return true
	})

	ic.soundPlaying = true
}

// StopAllSounds sends stop sound command to all devices with active connections
func (ic *InteractiveController) StopAllSounds() {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	ic.devices.Range(func(key, value interface{}) bool {
		info := value.(*DeviceInfo)

		// Only try devices that have the characteristic ready
		if info.ConnectStatus != "ready" || info.Characteristic == nil {
			return true
		}

		// Send stop command using existing connection
		cmd := make([]byte, 2)
		cmd[0] = byte(OpcodeSoundStop & 0xFF)
		cmd[1] = byte(OpcodeSoundStop >> 8)

		info.Characteristic.WriteWithoutResponse(cmd)
		return true
	})

	ic.soundPlaying = false
}

// ToggleSound toggles between play and stop
func (ic *InteractiveController) ToggleSound() {
	if ic.soundPlaying {
		ic.StopAllSounds()
	} else {
		ic.TriggerAllSounds()
	}
}

// IsSoundPlaying returns current sound state
func (ic *InteractiveController) IsSoundPlaying() bool {
	return ic.soundPlaying
}

// StartInputListener listens for keyboard input
// 's' = scan mode, 'c' = connect mode, Enter = play/stop sound (in connect mode)
func (ic *InteractiveController) StartInputListener(quit chan any) {
	reader := bufio.NewReader(os.Stdin)

	for {
		select {
		case <-quit:
			return
		default:
			input, err := reader.ReadString('\n')
			if err != nil {
				continue
			}

			// Trim the newline and check input
			input = input[:len(input)-1]
			if len(input) > 0 && input[len(input)-1] == '\r' {
				input = input[:len(input)-1]
			}

			switch input {
			case "s", "S":
				ic.switchToScanMode()
			case "c", "C":
				ic.switchToConnectMode()
			case "":
				// Enter key pressed
				if ic.mode == ModeConnect && ic.GetDeviceCount() > 0 {
					ic.ToggleSound()
				}
			}
		}
	}
}

// switchToScanMode switches to scan mode and resumes scanning
func (ic *InteractiveController) switchToScanMode() {
	if ic.mode == ModeScan {
		return
	}
	ic.mode = ModeScan
	if ic.onStartScan != nil {
		ic.onStartScan()
	}
}

// switchToConnectMode switches to connect mode, stops scanning, and tests connections
func (ic *InteractiveController) switchToConnectMode() {
	if ic.mode == ModeConnect {
		return
	}

	// Stop scanning first
	if ic.onStopScan != nil {
		ic.onStopScan()
	}

	ic.mode = ModeConnect

	// Test connections on all pending devices
	ic.testAllConnections()
}

// testAllConnections tests connections on all devices with pending status (sequential)
func (ic *InteractiveController) testAllConnections() {
	// Collect devices to test
	var toTest []*DeviceInfo
	var addrs []string

	ic.devices.Range(func(key, value interface{}) bool {
		addrStr := key.(string)
		info := value.(*DeviceInfo)

		if info.ConnectStatus == "pending" {
			info.ConnectStatus = "queued"
			toTest = append(toTest, info)
			addrs = append(addrs, addrStr)
		}
		return true
	})

	// Test one at a time (BLE stacks often can't handle parallel connections)
	for i, info := range toTest {
		info.ConnectStatus = "testing"
		ic.testConnection(addrs[i], info)
	}
}
