package main

import (
	"bytes"
	"fmt"
	"log"
	"reflect"
	"sort"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

const (
	scanRate                 = 500 * time.Millisecond // rate at which to scan for devices.
	scanBufferSize           = 500                    // buffer size for the scan channel.
	scanLength               = 200 * time.Millisecond // length of time to scan for devices.
	writeTime                = 10 * time.Second       // rate at which to write devices to the ingest path.
	trimTime                 = 1 * time.Second        // rate at which to trim the map of old devices.
	oldestDevice             = 24 * time.Hour         // time to keep a device in the map.
	adManSpecData            = byte(0xFF)             // 0xFF is the AD type for manufacturer specific data.
	appleIdentifier          = byte(0x004C)           // 0x004C is the company identifier for Apple.
	findMyNetworkBroadcastID = byte(0x12)             // 0x12 is the broadcast ID for the FindMy network.
	unregisteredFindMyDevice = byte(0x07)             // 0x07 is the broadcast ID for the FindMy network broadcast by an unregistered airtag.
	AirTagPayloadLength      = byte(0x19)             // 0x19 is the length of the AirTag payload.
)

var (
	lastSent []device
	findMy   map[string][]byte = map[string][]byte{
		"payloadType":   {unregisteredFindMyDevice, findMyNetworkBroadcastID},
		"payloadLength": {AirTagPayloadLength},
	}
)

type scanner struct {
	wg        *sync.WaitGroup    // WaitGroup to wait for the scan to finish.
	adptr     *bluetooth.Adapter // The adapter to use for scanning.
	devices   *sync.Map          // The map to store the devices.
	count     int                // The number of devices found.
	start     time.Time          // The time the scan started.
	quit      chan any           // Channel to signal the scan to stop.
	ingPath   ingestPath         // Channel to ingest the devices.
	scanCount int                // The number of scans that have been performed.
}

// device content
type device struct {
	d         bluetooth.ScanResult
	lastSeen  time.Time
	firstSeen time.Time
	timesSeen int
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

// store for bluetooth device Manufacturer specific data
type manData map[uint16][]byte

// ingestion path for devices.
type ingestPath chan deviceList

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

// Returns the device.
func (d device) Device() bluetooth.ScanResult {
	return d.d
}

// Returns the device address.
func (d device) Address() bluetooth.Address {
	return d.d.Address
}

// Returns the device address as a string.
func (d device) AddressString() string {
	return d.d.Address.String()
}

func (d device) ManufacturerData() map[uint16][]byte {
	return d.d.ManufacturerData()
}

// returns the device's local name.
func (d device) LocalName() string {
	return d.d.LocalName()
}

// returns the device's company uint16 identifier.
func (d device) CompanyIdent() uint16 {
	return getCompanyIdent(d.ManufacturerData())
}

// Active scanner. scans for new devices and passes them back down it's return path.
func (s *scanner) scan(returnPath chan bluetooth.ScanResult, writeTrigger chan any) {
	log.Printf("[DEBUG] Starting active scanner loop")
	s.scanCount = 0
	for {
		// set a new timer to start scanning.
		startScanTimer := time.NewTimer(scanRate)
		defer startScanTimer.Stop()
		select {
		case <-s.quit:
			log.Printf("[DEBUG] Scanner received quit signal, stopping scan loop")
			s.wg.Done()
			return
		case <-startScanTimer.C: // start scanning for devices.
			log.Printf("[DEBUG] Starting scan cycle #%d", s.scanCount+1)
			stopScanTimer := time.NewTimer(scanLength)
			defer stopScanTimer.Stop()
			err := s.adptr.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
				select {
				case <-stopScanTimer.C:
					s.scanCount++
					log.Printf("[DEBUG] Scan cycle #%d completed, triggering write operation", s.scanCount)
					writeTrigger <- interface{}(nil)
					adapter.StopScan()
					return
				default:
					log.Printf("[DEBUG] Device detected: %s, RSSI: %d", device.Address.String(), device.RSSI)
					returnPath <- device // pass the device back to the scanner.
				}
			})
			if err != nil {
				log.Printf("[DEBUG] Scan error occurred: %v", err)
				log.Printf("failed to scan: %v\n", err)
			}

		}
	}
}

// returns a new scan devices.
func newScanner(wg *sync.WaitGroup, adptr *bluetooth.Adapter, devices *sync.Map, q chan any) *scanner {
	log.Printf("[DEBUG] Creating new scanner instance")
	return &scanner{wg: wg, adptr: adptr, devices: devices, quit: q}
}

// Primary operation block of the scanner.
// Starts the scanner, listens for devices on the return path, stores them in map
// periodically cleans up the map, and passes a sorted list of devices to the writer.
func (s *scanner) startScan() {
	log.Printf("[DEBUG] Starting primary scan operation block")
	s.count = 0
	returnPath := make(chan bluetooth.ScanResult, scanBufferSize)
	writeTrigger := make(chan any, 1)
	trimTicker := time.NewTicker(trimTime)
	log.Printf("[DEBUG] Created channels and ticker - buffer size: %d, trim interval: %v", scanBufferSize, trimTime)

	go s.scan(returnPath, writeTrigger)
	log.Printf("[DEBUG] Launched scanner goroutine")

	for {
		select {
		// check for the signal to stop scanning.
		case <-s.quit:
			log.Printf("[DEBUG] Primary scan loop received quit signal")
			s.wg.Done()
			return
		// recieve devices from the scanner and store them in the map.
		case dev := <-returnPath:
			log.Printf("[DEBUG] Processing device from return path: %s", dev.Address.String())
			devicesEntry := device{
				d: dev,
			}
			// if the device is not an Apple FindMy device, skip it.
			if !devicesEntry.isTrackingAirtag() {
				log.Printf("[DEBUG] Device %s is not a tracking AirTag, skipping", dev.Address.String())
				continue
			}
			log.Printf("[DEBUG] Device %s identified as tracking AirTag", dev.Address.String())

			// if the device has been seen before, update the last seen time and increment the times seen.
			if value, ok := s.devices.Load(dev.Address.String()); ok {
				log.Printf("[DEBUG] Updating existing device %s", dev.Address.String())
				deviceEntry := value.(map[string]device)[dev.Address.String()]
				oldTimesSeen := deviceEntry.timesSeen
				deviceEntry.lastSeen = time.Now()
				deviceEntry.timesSeen++
				s.devices.Store(dev.Address.String(), map[string]device{
					dev.Address.String(): deviceEntry,
				})
				log.Printf("[DEBUG] Device %s updated - times seen: %d -> %d", dev.Address.String(), oldTimesSeen, deviceEntry.timesSeen)
				continue
			}
			// if the device is new, add it to the map.
			log.Printf("[DEBUG] Adding new device %s to tracking map", dev.Address.String())
			s.devices.Store(dev.Address.String(), map[string]device{
				dev.Address.String(): {
					d:         dev,
					lastSeen:  time.Now(),
					firstSeen: time.Now(),
					timesSeen: 1,
				},
			})
			// increment the count of devices.
			s.count++
			log.Printf("[DEBUG] Total unique devices tracked: %d", s.count)
		// pass a list of devices to the writer.
		case <-writeTrigger:
			log.Printf("[DEBUG] Write trigger received, preparing device list")
			sendList := s.sortAndPass()
			sendList.scanCount = s.scanCount
			log.Printf("[DEBUG] Sorted device list prepared - %d devices, scan count: %d", len(sendList.devices), sendList.scanCount)

			// only send the list if it has changed.
			if !areSlicesEqual(sendList.devices, lastSent) {
				log.Printf("[DEBUG] Device list has changed, sending to writer")
				lastSent = sendList.devices
				s.ingPath <- sendList
			} else {
				log.Printf("[DEBUG] Device list unchanged, skipping write")
			}
		// start cleaning up the map of old devices.
		case <-trimTicker.C:
			log.Printf("[DEBUG] Trim ticker fired, cleaning up old devices")
			s.TrimMap()
		}

	}
}

// Boot-straping routine for the BLE scanner.
func startBleScanner(wg *sync.WaitGroup, ingPath ingestPath, q chan any) error {
	log.Printf("[DEBUG] Starting BLE scanner bootstrap routine")
	d := new(sync.Map)
	adapter := bluetooth.DefaultAdapter
	log.Printf("[DEBUG] Attempting to enable Bluetooth adapter")
	err := adapter.Enable()
	if err != nil {
		log.Printf("[DEBUG] Failed to enable Bluetooth adapter: %v", err)
		return fmt.Errorf("failed to enable bluetooth adapter: %v", err)
	}
	log.Printf("[DEBUG] Bluetooth adapter enabled successfully")

	scan := newScanner(wg, adapter, d, q)
	scan.ingPath = ingPath
	scan.start = time.Now()
	log.Printf("[DEBUG] Scanner configured with ingest path, start time: %v", scan.start)

	go func() {
		// start scanning for devices
		log.Printf("[DEBUG] Starting scanner in dedicated goroutine")
		scan.startScan()
	}()
	log.Printf("[DEBUG] BLE scanner bootstrap completed successfully")
	return nil
}

// cleans up stale devices from the map.
func (s *scanner) TrimMap() {
	log.Printf("[DEBUG] Starting device map trim operation")
	removed := 0
	s.devices.Range(func(k, v interface{}) bool {
		for _, dv := range v.(map[string]device) {
			timeSinceLastSeen := time.Since(dv.lastSeen)
			if timeSinceLastSeen > oldestDevice {
				log.Printf("[DEBUG] Removing stale device %s (last seen %v ago)", k, timeSinceLastSeen)
				s.devices.Delete(k)
				removed++
			}
		}
		return true
	})
	s.count -= removed
	log.Printf("[DEBUG] Trim operation completed - removed %d devices, total remaining: %d", removed, s.count)
}

// returns a sorted list of devices.
func (s *scanner) sortAndPass() deviceList {
	log.Printf("[DEBUG] Creating sorted device list")
	sortedList := deviceList{}
	s.devices.Range(func(k, v interface{}) bool {
		for _, dv := range v.(map[string]device) {
			sortedList.devices = append(sortedList.devices, dv)
		}
		return true
	})
	log.Printf("[DEBUG] Collected %d devices for sorting", len(sortedList.devices))
	sort.Sort(sortedList)
	log.Printf("[DEBUG] Device list sorted successfully")
	// return sorted list by device id
	return sortedList
}

// compares and returns true if the two []devices slices are equal.
func areSlicesEqual(listOne, listTwo []device) bool {
	equal := reflect.DeepEqual(listOne, listTwo)
	log.Printf("[DEBUG] Device list comparison result: %t", equal)
	return equal
}

// Checks if a device is potentiall an Apple AirTag.
func (d *device) isAppleAirTag() bool {
	log.Printf("[DEBUG] Checking if device %s is Apple AirTag", d.d.Address.String())
	if len(d.ManufacturerData()) == 0 {
		log.Printf("[DEBUG] Device %s has no manufacturer data", d.d.Address.String())
		return false
	}
	if val, ok := d.ManufacturerData()[uint16(appleIdentifier)]; ok {
		log.Printf("[DEBUG] Device %s has Apple manufacturer data (length: %d)", d.d.Address.String(), len(val))
		if len(val) > 0 {
			// check if the first byte is a FindMy network broadcast ID. And the second byte is the correct payload length.
			hasCorrectType := bytes.Contains(findMy["payloadType"], val[0:1])
			hasCorrectLength := bytes.Equal(findMy["payloadLength"], val[1:2])
			log.Printf("[DEBUG] Device %s - correct type: %t, correct length: %t (first byte: 0x%02X)",
				d.d.Address.String(), hasCorrectType, hasCorrectLength, val[0])
			if hasCorrectType && hasCorrectLength {
				log.Printf("[DEBUG] Device %s confirmed as Apple AirTag", d.d.Address.String())
				return true
			}
		}
	} else {
		log.Printf("[DEBUG] Device %s does not have Apple manufacturer data", d.d.Address.String())
	}
	return false
}

// Returns true if the device is a registered apple air tag
func (d device) isTrackingAirtag() bool {
	isRegistered := d.isRegistered()
	isAirTag := d.isAppleAirTag()
	log.Printf("[DEBUG] Device %s tracking check - registered: %t, AirTag: %t",
		d.d.Address.String(), isRegistered, isAirTag)
	if isRegistered && isAirTag {
		log.Printf("[DEBUG] Device %s confirmed as tracking AirTag", d.d.Address.String())
		return true
	}
	return false
}

// Checks if a device is potentially an Apple "FindMy" device.
func (d *device) isFindMyDevice() bool {
	log.Printf("[DEBUG] Checking if device %s is FindMy device", d.d.Address.String())
	var findMy map[string][]byte = map[string][]byte{
		"payloadType":   {unregisteredFindMyDevice, findMyNetworkBroadcastID},
		"payloadLength": {AirTagPayloadLength},
	}
	// Check if the device is broadcasting any manufacterer specific data.
	if len(d.ManufacturerData()) == 0 {
		log.Printf("[DEBUG] Device %s has no manufacturer data for FindMy check", d.d.Address.String())
		return false
	}
	// pulls Apple manufacturer data from the device.
	if val, ok := d.ManufacturerData()[uint16(appleIdentifier)]; ok {
		if len(val) > 0 {
			// Looks for a "findMy" AD type.
			if bytes.Contains(findMy["payloadType"], val[0:1]) {
				log.Printf("[DEBUG] Device %s confirmed as FindMy device", d.d.Address.String())
				return true
			}
		}
	}
	log.Printf("[DEBUG] Device %s is not a FindMy device", d.d.Address.String())
	return false
}

// Check if AirTag is registered or unregistered.
func (d device) isRegistered() bool {
	log.Printf("[DEBUG] Checking registration status for device %s", d.d.Address.String())
	if len(d.ManufacturerData()) == 0 || !d.isAppleAirTag() {
		log.Printf("[DEBUG] Device %s cannot check registration - no data or not AirTag", d.d.Address.String())
		return false
	}
	isRegistered := d.ManufacturerData()[uint16(appleIdentifier)][0] != unregisteredFindMyDevice
	log.Printf("[DEBUG] Device %s registration status: %t (first byte: 0x%02X)",
		d.d.Address.String(), isRegistered, d.ManufacturerData()[uint16(appleIdentifier)][0])
	return isRegistered
}
