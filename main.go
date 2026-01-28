package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sync"

	"tinygo.org/x/bluetooth"
)

var (
	companyNames    = make(CompanyIdentifiers)
	noFilter        = flag.Bool("no-filter", false, "disable all device filters (show all BLE devices)")
	playSound       = flag.String("play-sound", "", "trigger sound on device with given MAC address (e.g., AA:BB:CC:DD:EE:FF)")
	stopSound       = flag.String("stop-sound", "", "stop sound on device with given MAC address")
	interactiveMode = flag.Bool("interactive", false, "interactive mode: press Enter to toggle sound on all detected AirTags")

	// Global interactive controller (used when -interactive flag is set)
	interactiveController *InteractiveController
)

func main() {
	flag.Parse()

	// Handle sound trigger mode (single device)
	if *playSound != "" || *stopSound != "" {
		runSoundCommand()
		return
	}

	// Normal/Interactive scanning mode
	if *noFilter {
		SetFilters(FilterNone)
	}

	companyNames = loadCompanyIdentifiers(companyIdentifiersFile)
	deviceChan := make(DeviceChannel)
	quit := make(chan any)
	wg := sync.WaitGroup{}

	// Enable Bluetooth adapter once (shared between scanner and interactive controller)
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		log.Fatalf("Failed to enable Bluetooth adapter: %v", err)
	}

	// Initialize interactive controller if in interactive mode
	if *interactiveMode {
		interactiveController = NewInteractiveController(adapter)

		// Set callbacks for scan control
		interactiveController.SetScanCallbacks(PauseScanning, ResumeScanning)

		// Set callback to register devices as they're discovered
		DeviceCallback = func(address bluetooth.Address) {
			interactiveController.RegisterDevice(address)
		}

		// Start input listener in a goroutine
		go interactiveController.StartInputListener(quit)
	}

	wg.Add(2)

	go func() {
		err := startBleScannerWithAdapter(&wg, adapter, deviceChan, quit)
		mustSucceed("start Bluetooth scanner", err)
	}()

	go func() {
		err := startWriter(&wg, quit, os.Stdout, header, deviceChan)
		mustSucceed("start display writer", err)
	}()

	wg.Wait()
	log.Printf("Scanner and writer have finished.")
}

// runSoundCommand handles the sound trigger/stop commands
func runSoundCommand() {
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		log.Fatalf("Failed to enable Bluetooth adapter: %v", err)
	}

	trigger := NewSoundTrigger(adapter)

	if *playSound != "" {
		address, err := parseAddress(*playSound)
		if err != nil {
			log.Fatalf("Invalid MAC address '%s': %v", *playSound, err)
		}

		if err := trigger.TriggerSound(address); err != nil {
			log.Fatalf("Failed to trigger sound: %v", err)
		}
		log.Println("Sound triggered successfully!")
	}

	if *stopSound != "" {
		address, err := parseAddress(*stopSound)
		if err != nil {
			log.Fatalf("Invalid MAC address '%s': %v", *stopSound, err)
		}

		if err := trigger.StopSound(address); err != nil {
			log.Fatalf("Failed to stop sound: %v", err)
		}
		log.Println("Sound stopped successfully!")
	}
}

// parseAddress converts a MAC address string to a bluetooth.Address
func parseAddress(macStr string) (bluetooth.Address, error) {
	var addr bluetooth.Address
	addr.Set(macStr)
	// Verify it parsed correctly by checking if it's non-zero
	if addr.String() == "00:00:00:00:00:00" && macStr != "00:00:00:00:00:00" {
		return bluetooth.Address{}, fmt.Errorf("failed to parse MAC address")
	}
	return addr, nil
}
