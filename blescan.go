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
	scanRate       = 50 * time.Millisecond  // rate at which to scan for devices.
	scanBufferSize = 500                    // buffer size for the scan channel.
	scanLength     = 200 * time.Millisecond // length of time to scan for devices.
	writeTime      = 10 * time.Second       // rate at which to write devices to the ingest path.
	trimTime       = 1 * time.Second        // rate at which to trim the map of old devices.
	oldestDevice   = 24 * time.Hour         // time to keep a device in the map.
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
	wg          *sync.WaitGroup    // WaitGroup to wait for the scan to finish.
	adapter     *bluetooth.Adapter // The Bluetooth adapter for scanning.
	devices     *sync.Map          // Thread-safe map of discovered devices.
	deviceCount int                // Number of unique devices found.
	startTime   time.Time          // When scanning started.
	quit        chan any           // Channel to signal shutdown.
	outputChan  DeviceChannel      // Channel to send device lists to writer.
	scanCount   int                // Number of scan cycles completed.
}

// device wraps a BLE scan result with tracking metadata.
type device struct {
	scanResult bluetooth.ScanResult
	lastSeen   time.Time
	firstSeen  time.Time
	timesSeen  int
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

// list of devices
type deviceList struct {
	devices   []device
	scanCount int
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

// Returns the number of times the device was seen.
func (d device) TimesSeen() int {
	return d.timesSeen
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

// scan continuously discovers BLE devices and sends them to discoveredDevices channel.
func (s *scanner) scan(discoveredDevices chan bluetooth.ScanResult, displayRefresh chan any) {
	s.scanCount = 0
	for {
		scanDelayTimer := time.NewTimer(scanRate)
		select {
		case <-s.quit:
			scanDelayTimer.Stop()
			s.wg.Done()
			return
		case <-scanDelayTimer.C:
			scanDurationTimer := time.NewTimer(scanLength)
			err := s.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
				select {
				case <-scanDurationTimer.C:
					s.scanCount++
					displayRefresh <- nil
					adapter.StopScan()
					return
				default:
					discoveredDevices <- result
				}
			})
			scanDurationTimer.Stop()
			if err != nil {
				log.Printf("failed to scan: %v\n", err)
			}
		}
	}
}

// newScanner creates a new BLE scanner instance.
func newScanner(wg *sync.WaitGroup, adapter *bluetooth.Adapter, devices *sync.Map, quit chan any) *scanner {
	return &scanner{wg: wg, adapter: adapter, devices: devices, quit: quit}
}

// startScan is the primary event loop for the scanner.
// It receives discovered devices, stores them, periodically cleans stale entries,
// and sends sorted snapshots to the display writer.
func (s *scanner) startScan() {
	s.deviceCount = 0
	discoveredDevices := make(chan bluetooth.ScanResult, scanBufferSize)
	displayRefresh := make(chan any, 1)
	cleanupTicker := time.NewTicker(trimTime)
	go s.scan(discoveredDevices, displayRefresh)

	for {
		select {
		case <-s.quit:
			s.wg.Done()
			return

		case result := <-discoveredDevices:
			discovered := device{scanResult: result}

			// Apply active filters - skip if device doesn't pass.
			if !discovered.passesFilters() {
				continue
			}

			addr := result.Address.String()

			// Update existing device or add new one.
			if existing, ok := s.devices.Load(addr); ok {
				entry := existing.(map[string]device)[addr]
				entry.lastSeen = time.Now()
				entry.timesSeen++
				s.devices.Store(addr, map[string]device{addr: entry})
				continue
			}

			// New device - add to map.
			s.devices.Store(addr, map[string]device{
				addr: {
					scanResult: result,
					lastSeen:   time.Now(),
					firstSeen:  time.Now(),
					timesSeen:  1,
				},
			})
			s.deviceCount++

		case <-displayRefresh:
			snapshot := s.getSortedDevices()
			snapshot.scanCount = s.scanCount
			s.outputChan <- snapshot

		case <-cleanupTicker.C:
			s.removeStaleDevices()
		}
	}
}

// startBleScanner initializes and starts the BLE scanner.
func startBleScanner(wg *sync.WaitGroup, output DeviceChannel, quit chan any) error {
	deviceMap := new(sync.Map)
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return fmt.Errorf("failed to enable bluetooth adapter: %v", err)
	}

	bleScanner := newScanner(wg, adapter, deviceMap, quit)
	bleScanner.outputChan = output
	bleScanner.startTime = time.Now()

	go bleScanner.startScan()
	return nil
}

// removeStaleDevices cleans up devices not seen within the retention period.
func (s *scanner) removeStaleDevices() {
	removedCount := 0
	s.devices.Range(func(key, value interface{}) bool {
		for _, dev := range value.(map[string]device) {
			if time.Since(dev.lastSeen) > oldestDevice {
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


