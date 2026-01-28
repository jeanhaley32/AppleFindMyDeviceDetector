package main

import (
	"flag"
	"log"
	"os"
	"sync"
)

var (
	companyNames = make(CompanyIdentifiers)
	noFilter     = flag.Bool("no-filter", false, "disable all device filters (show all BLE devices)")
)

func main() {
	flag.Parse()

	if *noFilter {
		SetFilters(FilterNone)
	}

	companyNames = loadCompanyIdentifiers(companyIdentifiersFile)
	deviceChan := make(DeviceChannel)
	wg := sync.WaitGroup{}

	wg.Add(2)

	go func() {
		err := startBleScanner(&wg, deviceChan, make(chan any))
		mustSucceed("start Bluetooth scanner", err)
	}()

	go func() {
		err := startWriter(&wg, make(chan any), os.Stdout, header, deviceChan)
		mustSucceed("start display writer", err)
	}()

	wg.Wait()
	log.Printf("Scanner and writer have finished.")
}
