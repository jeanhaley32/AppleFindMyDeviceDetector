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
	scanRate       = 200 * time.Millisecond // rate at which to scan for devices.
	scanBufferSize = 100                    // buffer size for the scan channel.
	scanLength     = 500 * time.Millisecond // length of time to scan for devices.
	writeTime      = 2 * time.Second        // rate at which to write devices to the ingest path.
	trimTime       = 5 * time.Second        // rate at which to trim the map of old devices.
	oldestDevice   = 60 * time.Second       // time to keep a device in the map.
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

type DevContentList []devContent

func (d DevContentList) Len() int {
	return len(d)
}

func (d DevContentList) Less(i, j int) bool {
	return d[i].id < d[j].id
}

func (d DevContentList) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

// store for bluetooth device Manufacturer specific data
type manData map[uint16][]byte

// Return path for found devices
type ingestPath chan []devContent

// struct defining an individual devices data
type devContent struct {
	id               string
	manufacturerData manData
	localName        string
	companyIdent     uint16
	lastSeen         time.Time
}

// active scanner, scans for devices and passes them to it's parent process.
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

// scan loop: scans for devices; passes them down the ingest path; sleeps and starts again.
func (s *scanner) startScan() {
	s.count = 0
	returnPath := make(chan bluetooth.ScanResult, scanBufferSize)
	writeTicker := time.NewTicker(writeTime) // ticker to write devices to the ingest path.
	trimTicker := time.NewTicker(trimTime)
	go s.scan(returnPath)
	for {
		select {
		case <-s.quit:
			s.wg.Done()
			return
		case device := <-returnPath:
			// check if the device.Address contains a MAC address or a UUID

			s.devices.Store(device, map[bluetooth.UUID]devContent{
				device.Address.UUID: {
					id:               device.Address.String(),
					manufacturerData: device.ManufacturerData(),
					localName:        device.LocalName(),
					companyIdent:     getCompanyIdent(device.ManufacturerData()),
					lastSeen:         time.Now(),
				},
			})
			s.count++
		case <-writeTicker.C:
			// pass the devices down the ingest path.
			s.ingPath <- s.sortAndPass()
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

// Trims the map of devices that have not been seen in the last <oldestDevice> time.
// modified count to reflect the number of devices removed.
func (s *scanner) TrimMap() {
	removed := 0
	s.devices.Range(func(k, v interface{}) bool {
		for _, dv := range v.(map[bluetooth.UUID]devContent) {
			if time.Since(dv.lastSeen) > oldestDevice {
				s.devices.Delete(k)
				removed++
			}
		}
		return true
	})
	s.count -= removed
}

func (s *scanner) sortAndPass() DevContentList {
	// sort devices by device ID
	// pass devices to ingest path
	sortedList := DevContentList{}
	s.devices.Range(func(k, v interface{}) bool {
		for _, dv := range v.(map[bluetooth.UUID]devContent) {
			sortedList = append(sortedList, dv)
		}
		return true
	})
	sort.Sort(sortedList)
	// return sorted list by device id
	return sortedList
}
