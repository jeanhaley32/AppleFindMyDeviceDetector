package scanner

import (
	"airtagtracker/internal/device"
	"log"
	"reflect"
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

var (
	lastSent []device.Device
)

type Scanner struct {
	wg        *sync.WaitGroup    // WaitGroup to wait for the scan to finish.
	adptr     *bluetooth.Adapter // The adapter to use for scanning.
	devices   *sync.Map          // The map to store the devices.
	count     int                // The number of devices found.
	start     time.Time          // The time the scan started.
	quit      chan any           // Channel to signal the scan to stop.
	ingPath   IngestPath         // Channel to ingest the devices.
	scanCount int                // The number of scans that have been performed.
}

// ingestion path for devices.
type IngestPath chan device.List

// Active scanner. scans for new devices and passes them back down it's return path.
func (s *Scanner) scan(returnPath chan bluetooth.ScanResult, writeTrigger chan any) {
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
func NewScanner(wg *sync.WaitGroup, adptr *bluetooth.Adapter, devices *sync.Map, q chan any) *Scanner {
	return &Scanner{wg: wg, adptr: adptr, devices: devices, quit: q}
}

// Primary operation block of the scanner.
// Starts the scanner, listens for devices on the return path, stores them in map
// periodically cleans up the map, and passes a sorted list of devices to the writer.
func (s *Scanner) startScan() {
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
		case dev := <-returnPath:
			devicesEntry := device.Device{
				D: dev,
			}
			// if the device is not an Apple FindMy device, skip it.
			if !devicesEntry.IsTrackingAirtag() {
				continue
			}
			// if the device has been seen before, update the last seen time and increment the times seen.
			if value, ok := s.devices.Load(dev.Address.String()); ok {
				deviceEntry := value.(map[string]device.Device)[dev.Address.String()]
				deviceEntry.LastSeen = time.Now()
				deviceEntry.TimesSeen++
				s.devices.Store(dev.Address.String(), map[string]device.Device{
					dev.Address.String(): deviceEntry,
				})
				continue
			}
			// if the device is new, add it to the map.
			s.devices.Store(dev.Address.String(), map[string]device.Device{
				dev.Address.String(): {
					D:         dev,
					LastSeen:  time.Now(),
					FirstSeen: time.Now(),
					TimesSeen: 1,
				},
			})
			// increment the count of devices.
			s.count++
		// pass a list of devices to the writer.
		case <-writeTrigger:
			sendList := s.sortAndPass()
			sendList.ScanCount = s.scanCount
			// only send the list if it has changed.
			if !areSlicesEqual(sendList.Devices, lastSent) {
				lastSent = sendList.Devices
				s.ingPath <- sendList
			}
		// start cleaning up the map of old devices.
		case <-trimTicker.C:
			s.TrimMap()
		}

	}
}

// Start boot-straping routine for the BLE scanner.
func (s *Scanner) Start(ingPath IngestPath) error {
	adapter := bluetooth.DefaultAdapter
	err := adapter.Enable()
	if err != nil {
		return err
	}
	s.ingPath = ingPath
	s.start = time.Now()
	// start scanning for devices
	s.startScan()
	return nil
}

// cleans up stale devices from the map.
func (s *Scanner) TrimMap() {
	removed := 0
	s.devices.Range(func(k, v interface{}) bool {
		for _, dv := range v.(map[string]device.Device) {
			if time.Since(dv.LastSeen) > oldestDevice {
				s.devices.Delete(k)
				removed++
			}
		}
		return true
	})
	s.count -= removed
}

// returns a sorted list of devices.
func (s *Scanner) sortAndPass() device.List {

	sortedList := device.List{}
	s.devices.Range(func(k, v interface{}) bool {
		for _, dv := range v.(map[string]device.Device) {
			sortedList.Devices = append(sortedList.Devices, dv)
		}
		return true
	})
	sort.Sort(sortedList)
	// return sorted list by device id
	return sortedList
}

// compares and returns true if the two []devices slices are equal.
func areSlicesEqual(listOne, listTwo []device.Device) bool {
	return reflect.DeepEqual(listOne, listTwo)
}



