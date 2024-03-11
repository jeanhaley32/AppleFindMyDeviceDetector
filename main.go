package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"time"

	"tinygo.org/x/bluetooth"
)

const (
	// The UUID of an Apple findmy device.
	appleIdentifier          = byte(0x004C) // 0x004C is the company identifier for Apple.
	findMyNetworkBroadcastID = byte(0x12)   // 0x12 is the broadcast ID for the FindMy network.
	scannerRunTime           = 10           // The number of seconds to run the scanner.
)

type trackingDevices map[[16]byte]map[uint16][]byte

var (
	adapter = bluetooth.DefaultAdapter
	devices = make(trackingDevices)
)

func main() {
	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())
	for {
		// Start scanning.
		fmt.Println("Scanning for nearby Bluetooth devices...")
		t := time.Now()
		i := 0
		err := adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
			if time.Now().After(t.Add(scannerRunTime * time.Second)) {
				fmt.Println("Scan time exceeded")
				adapter.StopScan() // Stop scanning after 10 seconds.
			}
			for k := range device.AdvertisementPayload.ManufacturerData() {
				// Check if the device is an Apple FindMy device.
				if k == uint16(appleIdentifier) {
					devices[device.Address.Bytes()] = device.AdvertisementPayload.ManufacturerData()

				}
			}
			i++
		})
		must("start scan", err)
		// Print the devices found.
		fmt.Println("Devices found:")
		clearScreen()
		for k, v := range devices {
			fmt.Printf("Device: %X, FindMyDevice: %v\n", k, isFindMyDevice(v))
		}
		// Clear the devices map.
		devices = make(trackingDevices)
		// Wait 5 seconds before scanning again.
		time.Sleep(5 * time.Second)
	}

}

// must is a helper function that wraps a call to a function returning an error and logs it if the error is non-nil.
func must(action string, err error) {
	if err != nil {
		log.Fatalf("Failed to %s: %v", action, err)
	}
}

// checks if byte 4 is the FindMy network broadcast ID.
func isFindMyDevice(b map[uint16][]byte) bool {
	if len(b) == 0 {
		return false
	}
	for _, v := range b {
		if len(b) < 5 {
			continue
		} else if v[4] == findMyNetworkBroadcastID {
			return true
		}
	}
	return false
}

func clearScreen() {
	cmd := exec.Command("clear") // Linux or macOS
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls") // Windows
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}
