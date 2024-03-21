package main

import (
	"fmt"
	"log"
	"reflect"
	"sort"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

const (
	scanRate       = 200 * time.Millisecond // rate at which to scan for devices.
	scanBufferSize = 100                    // buffer size for the scan channel.
	scanLength     = 500 * time.Millisecond // length of time to scan for devices.
	writeTime      = 2 * time.Second        // rate at which to write devices to the ingest path.
	trimTime       = 5 * time.Second        // rate at which to trim the map of old devices.
	oldestDevice   = 15 * time.Minute       // time to keep a device in the map.
)

var (
	lastSent []devContent
)

type scanner struct {
	wg      *sync.WaitGroup    // WaitGroup to wait for the scan to finish.
	adptr   *bluetooth.Adapter // The adapter to use for scanning.
	devices *sync.Map          // The map to store the devices.
	count   int                // The number of devices found.
	start   time.Time          // The time the scan started.
	quit    chan any           // Channel to signal the scan to stop.
	ingPath ingestPath         // Channel to ingest the devices.
}

// list of devices
type DevContentList []devContent

// returns the length of the list
// used to satisfy the sort.Interface
func (d DevContentList) Len() int {
	return len(d)
}

// return true if the device id is less than the device id at index j
// used to satisfy the sort.Interface
func (d DevContentList) Less(i, j int) bool {
	return d[i].id < d[j].id
}

// swaps the devices at index i and j
// used to satisfy the sort.Interface
func (d DevContentList) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

// store for bluetooth device Manufacturer specific data
type manData map[uint16][]byte

// ingestion path for devices.
type ingestPath chan []devContent

// device content
type devContent struct {
	id               string
	manufacturerData manData
	localName        string
	companyIdent     uint16
	lastSeen         time.Time
}

// Active scanner. scans for new devices and passes them back down it's return path.
func (s *scanner) scan(returnPath chan bluetooth.ScanResult) {
	// check for signal to stop scanning.
	for {
		startScanTimer := time.NewTimer(scanRate)
		select {
		case <-s.quit:
			s.wg.Done()
			return
		case <-startScanTimer.C:
			stopScanTimer := time.NewTimer(scanLength)
			err := s.adptr.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
				select {
				case <-stopScanTimer.C:
					adapter.StopScan()
					return
				default:
					returnPath <- device
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
	writeTicker := time.NewTicker(writeTime) // ticker to write devices to the ingest path.
	trimTicker := time.NewTicker(trimTime)
	go s.scan(returnPath)
	for {
		select {
		// check for the signal to stop scanning.
		case <-s.quit:
			s.wg.Done()
			return
		// recieve devices from the scanner and store them in the map.
		case device := <-returnPath:
			s.devices.Store(device.Address.String(), map[string]devContent{
				device.Address.String(): {
					id:               device.Address.String(),
					manufacturerData: device.ManufacturerData(),
					localName:        device.LocalName(),
					companyIdent:     getCompanyIdent(device.ManufacturerData()),
					lastSeen:         time.Now(),
				},
			})
			s.count++
		// pass a list of devices to the writer.
		case <-writeTicker.C:
			sendList := s.sortAndPass()
			// only send the list if it has changed.
			if !areSlicesEqual(sendList, lastSent) {
				lastSent = sendList
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
			sortedList = append(sortedList, dv)
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
