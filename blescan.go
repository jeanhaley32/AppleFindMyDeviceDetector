package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

const (
	scanWait = time.Duration(1 * time.Second)
	scanTime = time.Duration(5 * time.Second)
)

type scanDevs struct {
	wg       *sync.WaitGroup    // WaitGroup to wait for the scan to finish.
	scanTime time.Duration      // The duration to scan for devices.
	scanWait time.Duration      // The duration to wait between scans.
	adptr    *bluetooth.Adapter // The adapter to use for scanning.
	devices  *sync.Map          // The map to store the devices.
	start    time.Time          // The time the scan started.
	quit     chan struct{}      // Channel to signal the scan to stop.
	ingPath  ingestPath         // Channel to ingest the devices.
}

// store for bluetooth device Manufacturer specific data
type manData map[uint16][]byte

// Return path for found devices
type ingestPath chan *sync.Map

// struct defining an individual devices data
type devContent struct {
	manufacturerData manData
	localName        devName
	companyIdent     devValue
}

// populates the local device map by scanning for local BLE devices
func (s *scanDevs) scan() {
	// check for signal to stop scanning.
	fmt.Println("Scanning for nearby Bluetooth devices...")
	s.devices = &sync.Map{}
	t := time.Now()
	err := s.adptr.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		startScan := time.Now()                  // Time this scan started
		if time.Now().After(t.Add(s.scanTime)) { // If the scan time has elapsed then stop scanning
			fmt.Printf("scan complete after: %v\n", time.Since(startScan)) // Log the time it took to scan
			adapter.StopScan()
		}
		s.devices.Store(device.Address.Get16Bit(), map[uint16]devContent{
			device.Address.Get16Bit(): {
				manufacturerData: device.ManufacturerData(),
				localName:        devName(device.LocalName()),
				companyIdent:     devValue(getCompanyIdent(device.ManufacturerData())),
			}})
	})
	if err != nil {
		log.Printf("failed to scan: %v\n", err)
	}
}

// returns a new scan devices.
func newScanDevs(wg *sync.WaitGroup, scanTime time.Duration, adptr *bluetooth.Adapter, devices *sync.Map) *scanDevs {
	return &scanDevs{wg: wg, scanTime: scanTime, adptr: adptr, devices: devices}
}

// scan loop: scans for devices; passes them down the ingest path; sleeps and starts again. 
func (s *scanDevs) startScan() {
	for {
		select {
		case <-s.quit:
			s.wg.Done()
			return
		default:
			s.scan()
			s.ingPath <- s.devices
			s.devices = &sync.Map{} // Clear the map for the next scan.
			time.Sleep(s.scanWait)
		}

	}
}

// Boot-straping routine for the BLE scanner. 
func startBleScanner(wg *sync.WaitGroup, ingPath *ingestPath) error {
	d := new(sync.Map)
	adapter := bluetooth.DefaultAdapter
	err := adapter.Enable()
	if err != nil {
		return fmt.Errorf("failed to enable bluetooth adapter: %v", err)
	}
	scan := newScanDevs(wg, scanTime, adapter, d)
	scan.ingPath = *ingPath
	wg.Add(1)
	scan.start = time.Now()
	go scan.startScan()
	wg.Wait()
	return nil
}
