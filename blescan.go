package main

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

const (
	scanBufferSize  = 500              // buffer size for discovered devices channel.
	displayInterval = 10 * time.Second // how often to refresh the display.
	cleanupInterval = 1 * time.Second  // how often to remove stale devices.
	deviceRetention = 24 * time.Hour   // how long to keep a device in memory.
)

// DeviceFilter is a function that returns true if the device should be included.
type DeviceFilter func(*device) bool

// Available filters - each returns true if the device passes the filter.
var (
	// FilterAppleAirTag passes devices matching AirTag signature (type=0x12, len=0x19).
	FilterAppleAirTag DeviceFilter = func(d *device) bool {
		return d.isAppleAirTag()
	}

	// FilterOwnerNearby passes AirTags with "maintained" bit set in status byte.
	// Note: This checks if owner connected within 15-min key rotation period.
	FilterOwnerNearby DeviceFilter = func(d *device) bool {
		return d.isOwnerNearby()
	}

	// FilterFindMyDevice passes any device with FindMy type byte (0x12).
	FilterFindMyDevice DeviceFilter = func(d *device) bool {
		return d.isFindMyDevice()
	}

	// FilterAppleDevice passes any device with Apple manufacturer data (0x004C).
	FilterAppleDevice DeviceFilter = func(d *device) bool {
		return d.hasAppleData()
	}

	// FilterAirPods passes Apple AirPods (type=0x07).
	FilterAirPods DeviceFilter = func(d *device) bool {
		return d.isAirPods()
	}

	// FilterHasManufacturerData passes devices broadcasting any manufacturer data.
	FilterHasManufacturerData DeviceFilter = func(d *device) bool {
		return len(d.ManufacturerData()) > 0
	}

	// FilterNone passes all devices (no filtering).
	FilterNone DeviceFilter = func(d *device) bool {
		return true
	}
)

// activeFilters defines which filters are applied to incoming devices.
// A device must pass ALL filters to be stored. Comment/uncomment to toggle.
var activeFilters = []DeviceFilter{
	FilterAppleAirTag, // type=0x12, len=0x19
	// FilterOwnerNearby,      // status byte "maintained" bit (owner nearby)
	// FilterFindMyDevice,     // any FindMy type (0x12)
	// FilterAppleDevice,      // any Apple device (0x004C)
	// FilterAirPods,          // AirPods only (0x07)
	// FilterHasManufacturerData,
	// FilterNone,
}

// SetFilters replaces the active filters with the provided filters.
func SetFilters(filters ...DeviceFilter) {
	activeFilters = filters
}

// passesFilters returns true if the device passes all active filters.
func (d *device) passesFilters() bool {
	for _, filter := range activeFilters {
		if !filter(d) {
			return false
		}
	}
	return true
}

type scanner struct {
	wg           *sync.WaitGroup    // WaitGroup to wait for the scan to finish.
	adapter      *bluetooth.Adapter // The Bluetooth adapter for scanning.
	devices      *sync.Map          // Thread-safe map of discovered devices.
	deviceCount  int                // Number of unique devices found.
	startTime    time.Time          // When scanning started.
	quit         chan any           // Channel to signal shutdown.
	outputChan   DeviceChannel      // Channel to send device lists to writer.
	refreshCount int                // Number of display refreshes (for stats).
	paused       bool               // Whether scanning is paused.
	pauseChan    chan bool          // Channel to signal pause/resume.
}

// Global scanner instance for interactive mode control
var activeScanner *scanner

// PauseScanning stops the BLE scan temporarily
func PauseScanning() {
	if activeScanner != nil && !activeScanner.paused {
		activeScanner.adapter.StopScan()
		activeScanner.paused = true
	}
}

// ResumeScanning restarts the BLE scan
func ResumeScanning() {
	if activeScanner != nil && activeScanner.paused {
		activeScanner.paused = false
		go activeScanner.runContinuousScan(make(chan bluetooth.ScanResult, scanBufferSize))
	}
}

// device wraps a BLE scan result with tracking metadata.
type device struct {
	scanResult      bluetooth.ScanResult
	lastSeen        time.Time
	firstSeen       time.Time
	timesSeen       int // total BLE packets received
	refreshesSeen   int // number of refresh cycles device was seen in
	lastRefreshSeen int // last refresh cycle this device was seen in
}

// time since first seen
func (d device) sinceFirstSeen() time.Duration {
	return time.Since(d.firstSeen)
}

// time since last seen
func (d device) sinceLastSeen() time.Duration {
	return time.Since(d.lastSeen)
}

// returns the time since the device was first detected and the last time it was detected.
func (d device) detectedFor() time.Duration {
	return d.lastSeen.Sub(d.firstSeen)
}

// deviceList holds a snapshot of devices for display.
type deviceList struct {
	devices      []device
	refreshCount int
}

// returns the length of the list
// used to satisfy the sort.Interface
func (d deviceList) Len() int {
	return len(d.devices)
}

// return true if the device id is less than the device id at index j
// used to satisfy the sort.Interface
func (d deviceList) Less(i, j int) bool {
	return d.devices[j].detectedFor() < d.devices[i].detectedFor()
}

// swaps the devices at index i and j
// used to satisfy the sort.Interface
func (d deviceList) Swap(i, j int) {
	d.devices[i], d.devices[j] = d.devices[j], d.devices[i]
}

// ManufacturerData maps company IDs to their payload bytes.
type ManufacturerData map[uint16][]byte

// DeviceChannel is used to pass device lists between scanner and writer.
type DeviceChannel chan deviceList

// Returns the first time the device was seen.
func (d device) FirstSeen() time.Time {
	return d.firstSeen
}

// Returns the last time the device was seen.
func (d device) LastSeen() time.Time {
	return d.lastSeen
}

// Returns the number of times the device was seen (BLE packets).
func (d device) TimesSeen() int {
	return d.timesSeen
}

// Returns the number of refresh cycles the device was seen in.
func (d device) RefreshesSeen() int {
	return d.refreshesSeen
}

// Returns the underlying BLE scan result.
func (d device) ScanResult() bluetooth.ScanResult {
	return d.scanResult
}

// Returns the device address.
func (d device) Address() bluetooth.Address {
	return d.scanResult.Address
}

// Returns the device address as a string.
func (d device) AddressString() string {
	return d.scanResult.Address.String()
}

func (d device) ManufacturerData() map[uint16][]byte {
	return d.scanResult.ManufacturerData()
}

// returns the device's local name.
func (d device) LocalName() string {
	return d.scanResult.LocalName()
}

// CompanyID returns the Bluetooth company identifier from manufacturer data.
func (d device) CompanyID() uint16 {
	return extractCompanyID(d.ManufacturerData())
}

// runContinuousScan starts the BLE scan and forwards discovered devices to the channel.
// This function blocks until the adapter stops scanning (via StopScan).
func (s *scanner) runContinuousScan(discovered chan<- bluetooth.ScanResult) {
	err := s.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		select {
		case discovered <- result:
			// Device sent to processing channel
		default:
			// Channel full - silently drop to avoid blocking the BLE stack
		}
	})
	if err != nil {
		log.Printf("scan error: %v\n", err)
	}
}

// newScanner creates a new BLE scanner instance.
func newScanner(wg *sync.WaitGroup, adapter *bluetooth.Adapter, devices *sync.Map, quit chan any) *scanner {
	return &scanner{wg: wg, adapter: adapter, devices: devices, quit: quit}
}

// run is the main event loop for the scanner.
// It processes discovered devices, refreshes the display periodically,
// and cleans up stale entries.
func (s *scanner) run() {
	discovered := make(chan bluetooth.ScanResult, scanBufferSize)
	displayTicker := time.NewTicker(displayInterval)
	cleanupTicker := time.NewTicker(cleanupInterval)

	// Start continuous BLE scan in background
	go s.runContinuousScan(discovered)

	for {
		select {
		case <-s.quit:
			s.adapter.StopScan()
			displayTicker.Stop()
			cleanupTicker.Stop()
			s.wg.Done()
			return

		case result := <-discovered:
			s.processDevice(result)

		case <-displayTicker.C:
			s.refreshDisplay()

		case <-cleanupTicker.C:
			s.removeStaleDevices()
		}
	}
}

// DeviceCallback is called when a new device is discovered (for interactive mode)
var DeviceCallback func(address bluetooth.Address)

// processDevice handles a newly discovered BLE device.
func (s *scanner) processDevice(result bluetooth.ScanResult) {
	dev := device{scanResult: result}

	// Apply active filters
	if !dev.passesFilters() {
		return
	}

	addr := result.Address.String()

	// Register with interactive controller if callback is set
	if DeviceCallback != nil {
		DeviceCallback(result.Address)
	}

	// Update existing device or add new one
	if existing, ok := s.devices.Load(addr); ok {
		entry := existing.(map[string]device)[addr]
		entry.scanResult = result // Update with latest BLE data (status byte, etc.)
		entry.lastSeen = time.Now()
		entry.timesSeen++
		// Track if this is a new refresh cycle
		if entry.lastRefreshSeen < s.refreshCount {
			entry.refreshesSeen++
			entry.lastRefreshSeen = s.refreshCount
		}
		s.devices.Store(addr, map[string]device{addr: entry})
		return
	}

	// New device
	s.devices.Store(addr, map[string]device{
		addr: {
			scanResult:      result,
			lastSeen:        time.Now(),
			firstSeen:       time.Now(),
			timesSeen:       1,
			refreshesSeen:   1,
			lastRefreshSeen: s.refreshCount,
		},
	})
	s.deviceCount++
}

// refreshDisplay sends the current device list to the display writer.
func (s *scanner) refreshDisplay() {
	s.refreshCount++
	snapshot := s.getSortedDevices()
	snapshot.refreshCount = s.refreshCount
	s.outputChan <- snapshot
}

// startBleScanner initializes and starts the BLE scanner.
func startBleScanner(wg *sync.WaitGroup, output DeviceChannel, quit chan any) error {
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return fmt.Errorf("failed to enable bluetooth adapter: %v", err)
	}
	return startBleScannerWithAdapter(wg, adapter, output, quit)
}

// startBleScannerWithAdapter starts the BLE scanner with a pre-enabled adapter.
func startBleScannerWithAdapter(wg *sync.WaitGroup, adapter *bluetooth.Adapter, output DeviceChannel, quit chan any) error {
	s := newScanner(wg, adapter, new(sync.Map), quit)
	s.outputChan = output
	s.startTime = time.Now()
	activeScanner = s // Set global for interactive mode control

	go s.run()
	return nil
}

// removeStaleDevices cleans up devices not seen within the retention period.
func (s *scanner) removeStaleDevices() {
	removedCount := 0
	s.devices.Range(func(key, value interface{}) bool {
		for _, dev := range value.(map[string]device) {
			if time.Since(dev.lastSeen) > deviceRetention {
				s.devices.Delete(key)
				removedCount++
			}
		}
		return true
	})
	s.deviceCount -= removedCount
}

// getSortedDevices returns all devices sorted by detection duration (longest first).
func (s *scanner) getSortedDevices() deviceList {
	sorted := deviceList{}
	s.devices.Range(func(key, value interface{}) bool {
		for _, dev := range value.(map[string]device) {
			sorted.devices = append(sorted.devices, dev)
		}
		return true
	})
	sort.Sort(sorted)
	return sorted
}


