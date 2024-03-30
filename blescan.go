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
	scanRate                 = 50 * time.Millisecond  // rate at which to scan for devices.
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
	lastSent []devContent
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

// list of devices
type DevContentList struct {
	devContent []devContent
	scanCount  int
}

// returns the length of the list
// used to satisfy the sort.Interface
func (d DevContentList) Len() int {
	return len(d.devContent)
}

// return true if the device id is less than the device id at index j
// used to satisfy the sort.Interface
func (d DevContentList) Less(i, j int) bool {
	return d.devContent[j].firstSeen.After(d.devContent[i].firstSeen)
}

// swaps the devices at index i and j
// used to satisfy the sort.Interface
func (d DevContentList) Swap(i, j int) {
	d.devContent[i], d.devContent[j] = d.devContent[j], d.devContent[i]
}

// store for bluetooth device Manufacturer specific data
type manData map[uint16][]byte

// ingestion path for devices.
type ingestPath chan DevContentList

// device content
type devContent struct {
	device    bluetooth.ScanResult
	lastSeen  time.Time
	firstSeen time.Time
	timesSeen int
}

// Returns the first time the device was seen.
func (d devContent) FirstSeen() time.Time {
	return d.firstSeen
}

// Returns the last time the device was seen.
func (d devContent) LastSeen() time.Time {
	return d.lastSeen
}

// Returns the number of times the device was seen.
func (d devContent) TimesSeen() int {
	return d.timesSeen
}

// Returns the device.
func (d devContent) Device() bluetooth.ScanResult {
	return d.device
}

// Returns the device address.
func (d devContent) Address() bluetooth.Address {
	return d.device.Address
}

// Returns the device address as a string.
func (d devContent) AddressString() string {
	return d.device.Address.String()
}

func (d devContent) ManufacturerData() map[uint16][]byte {
	return d.device.ManufacturerData()
}

// returns the device's local name.
func (d devContent) LocalName() string {
	return d.device.LocalName()
}

// returns the device's company uint16 identifier.
func (d devContent) CompanyIdent() uint16 {
	return getCompanyIdent(d.ManufacturerData())
}

// Active scanner. scans for new devices and passes them back down it's return path.
func (s *scanner) scan(returnPath chan bluetooth.ScanResult, writeTrigger chan any) {
	s.scanCount = 0
	for {
		// set a new timer to start scanning.
		startScanTimer := time.NewTimer(scanRate)
		defer startScanTimer.Stop()
		select {
		case <-s.quit:
			s.wg.Done()
			return
		case <-startScanTimer.C: // start scanning for devices.
			stopScanTimer := time.NewTimer(scanLength)
			defer stopScanTimer.Stop()
			err := s.adptr.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
				select {
				case <-stopScanTimer.C:
					s.scanCount++
					writeTrigger <- interface{}(nil)
					adapter.StopScan()
					return
				default:
					returnPath <- device // pass the device back to the scanner.
				}
			})
			if err != nil {
				log.Printf("failed to scan: %v\n", err)
			}

		}
	}
}

// returns a new scan devices.
func newScanner(wg *sync.WaitGroup, adptr *bluetooth.Adapter, devices *sync.Map, q chan any) *scanner {
	return &scanner{wg: wg, adptr: adptr, devices: devices, quit: q}
}

// Primary operation block of the scanner.
// Starts the scanner, listens for devices on the return path, stores them in map
// periodically cleans up the map, and passes a sorted list of devices to the writer.
func (s *scanner) startScan() {
	s.count = 0
	returnPath := make(chan bluetooth.ScanResult, scanBufferSize)
	writeTrigger := make(chan any, 1)
	trimTicker := time.NewTicker(trimTime)
	go s.scan(returnPath, writeTrigger)
	for {
		select {
		// check for the signal to stop scanning.
		case <-s.quit:
			s.wg.Done()
			return
		// recieve devices from the scanner and store them in the map.
		case device := <-returnPath:
			devContentEntry := devContent{
				device: device,
			}
			// if the device is not an Apple FindMy device, skip it.
			if !devContentEntry.isFindMyDevice() {
				continue
			}
			// if the device has been seen before, update the last seen time and increment the times seen.
			if value, ok := s.devices.Load(device.Address.String()); ok {
				devContentEntry := value.(map[string]devContent)[device.Address.String()]
				devContentEntry.lastSeen = time.Now()
				devContentEntry.timesSeen++
				s.devices.Store(device.Address.String(), map[string]devContent{
					device.Address.String(): devContentEntry,
				})
				continue
			}
			// if the device is new, add it to the map.
			s.devices.Store(device.Address.String(), map[string]devContent{
				device.Address.String(): {
					device:    device,
					lastSeen:  time.Now(),
					firstSeen: time.Now(),
					timesSeen: 1,
				},
			})
			// increment the count of devices.
			s.count++
		// pass a list of devices to the writer.
		case <-writeTrigger:
			sendList := s.sortAndPass()
			sendList.scanCount = s.scanCount
			// only send the list if it has changed.
			if !areSlicesEqual(sendList.devContent, lastSent) {
				lastSent = sendList.devContent
				s.ingPath <- sendList
			}
		// start cleaning up the map of old devices.
		case <-trimTicker.C:
			s.TrimMap()
		}

	}
}

// Boot-straping routine for the BLE scanner.
func startBleScanner(wg *sync.WaitGroup, ingPath ingestPath, q chan any) error {
	d := new(sync.Map)
	adapter := bluetooth.DefaultAdapter
	err := adapter.Enable()
	if err != nil {
		return fmt.Errorf("failed to enable bluetooth adapter: %v", err)
	}
	scan := newScanner(wg, adapter, d, q)
	scan.ingPath = ingPath
	scan.start = time.Now()
	go func() {
		// start scanning for devices
		scan.startScan()
	}()
	return nil
}

// cleans up stale devices from the map.
func (s *scanner) TrimMap() {
	removed := 0
	s.devices.Range(func(k, v interface{}) bool {
		for _, dv := range v.(map[string]devContent) {
			if time.Since(dv.lastSeen) > oldestDevice {
				s.devices.Delete(k)
				removed++
			}
		}
		return true
	})
	s.count -= removed
}

// returns a sorted list of devices.
func (s *scanner) sortAndPass() DevContentList {

	sortedList := DevContentList{}
	s.devices.Range(func(k, v interface{}) bool {
		for _, dv := range v.(map[string]devContent) {
			sortedList.devContent = append(sortedList.devContent, dv)
		}
		return true
	})
	sort.Sort(sortedList)
	// return sorted list by device id
	return sortedList
}

// compares and returns true if the two []devContent slices are equal.
func areSlicesEqual(listOne, listTwo []devContent) bool {
	return reflect.DeepEqual(listOne, listTwo)
}

// Checks if a device is potentiall an Apple AirTag.
func (d *devContent) isAppleAirTag() bool {
	if len(d.ManufacturerData()) == 0 {
		return false
	}
	if val, ok := d.ManufacturerData()[uint16(appleIdentifier)]; ok {
		if len(val) > 0 {
			// check if the first byte is a FindMy network broadcast ID. And the second byte is the correct payload length.
			if bytes.Contains(findMy["payloadType"], val[0:1]) && bytes.Equal(findMy["payloadLength"], val[1:2]) {
				return true
			}
		}
	}
	return false
}

// Checks if a device is potentially an Apple "FindMy" device.
func (d *devContent) isFindMyDevice() bool {
	var findMy map[string][]byte = map[string][]byte{
		"payloadType":   {unregisteredFindMyDevice, findMyNetworkBroadcastID},
		"payloadLength": {AirTagPayloadLength},
	}
	// Check if the device is broadcasting any manufacterer specific data.
	if len(d.ManufacturerData()) == 0 {
		return false
	}
	// pulls Apple manufacturer data from the device.
	if val, ok := d.ManufacturerData()[uint16(appleIdentifier)]; ok {
		if len(val) > 0 {
			// Looks for a "findMy" AD type.
			if bytes.Contains(findMy["payloadType"], val[0:1]) {
				return true
			}
		}
	}
	return false
}

// Check if AirTag is registered or unregistered.
func (d devContent) isRegistered() bool {
	if len(d.ManufacturerData()) == 0 || !d.isAppleAirTag() {
		return false
	}
	return d.ManufacturerData()[uint16(appleIdentifier)][0] != unregisteredFindMyDevice
}
