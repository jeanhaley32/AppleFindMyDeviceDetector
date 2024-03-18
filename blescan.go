package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

const (
	deadline       = time.Duration(15 * time.Second) // deadline for the scan.
	scanBufferSize = 5                               // buffer size for the scan channel.
	scanRate       = 500 * time.Millisecond          // rate at which to scan for devices.
	writeTime      = 5 * time.Second                 // rate at which to write devices to the ingest path.
)

type scanner struct {
	wg      *sync.WaitGroup    // WaitGroup to wait for the scan to finish.
	adptr   *bluetooth.Adapter // The adapter to use for scanning.
	devices *sync.Map          // The map to store the devices.
	start   time.Time          // The time the scan started.
	quit    chan any           // Channel to signal the scan to stop.
	ingPath ingestPath         // Channel to ingest the devices.
}

// store for bluetooth device Manufacturer specific data
type manData map[uint16][]byte

// Return path for found devices
type ingestPath chan map[uint16]devContent

// struct defining an individual devices data
type devContent struct {
	manufacturerData manData
	localName        string
	companyIdent     uint16
	lastSeen         time.Time
}

// populates the local device map by scanning for local BLE devices
func (s *scanner) scan(returnPath chan bluetooth.ScanResult) {
	// check for signal to stop scanning.
	err := s.adptr.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		select {
		case <-s.quit:
			adapter.StopScan()
			return
		case <-time.After(scanRate):
			returnPath <- device
		}
	})
	if err != nil {
		scanlog(fmt.Sprintf("failed to scan: %v\n", err))
	}
}

// returns a new scan devices.
func newScanner(wg *sync.WaitGroup, adptr *bluetooth.Adapter, devices *sync.Map, q chan any) *scanner {
	return &scanner{wg: wg, adptr: adptr, devices: devices, quit: q}
}

// scan loop: scans for devices; passes them down the ingest path; sleeps and starts again.
func (s *scanner) startScan() {
	returnPath := make(chan bluetooth.ScanResult, scanBufferSize)
	writeTicker := time.NewTicker(writeTime) // ticker to write devices to the ingest path.
	go s.scan(returnPath)
	for {
		select {
		case <-s.quit:
			s.wg.Done()
			return
		case device := <-returnPath:
			// add the device to the map.
			s.devices.Store(device.Address.Get16Bit(), map[uint16]devContent{
				device.Address.Get16Bit(): {
					manufacturerData: device.ManufacturerData(),
					localName:        device.LocalName(),
					companyIdent:     getCompanyIdent(device.ManufacturerData()),
					lastSeen:         time.Now(),
				},
			})
		case <-writeTicker.C:
			// pass the devices down the ingest path.
			s.ingPath <- s.decoupleMap()
		case <-time.After(deadline):
			close(s.quit)
		}

	}
}

// Boot-straping routine for the BLE scanner.
func startBleScanner(wg *sync.WaitGroup, ingPath ingestPath, q chan any) error {
	log.Println("scanner: creating BLE scanner...")
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
		log.Println("scanner: starting BLE scanner...")
		scan.startScan()
	}()
	return nil
}

func (s *scanner) decoupleMap() map[uint16]devContent {
	deviceMap := make(map[uint16]devContent)
	s.devices.Range(func(k, v interface{}) bool {
		deviceMap[k.(uint16)] = v.(map[uint16]devContent)[k.(uint16)]
		return true
	})
	return deviceMap
}

func scanlog(s string) {
	log.Printf("Scanner: %v", s)
}

// func (s *scanner) trimOldDevices()
