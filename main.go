package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
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

	scannerQuit := make(chan any)
	writerQuit := make(chan any)

	// 3 goroutines call wg.Done(): scanner.scan(), scanner.startScan()
	// (both share scannerQuit), and writer.execute() (writerQuit).
	wg.Add(3)

	go func() {
		err := startBleScanner(&wg, deviceChan, scannerQuit)
		mustSucceed("start Bluetooth scanner", err)
	}()

	go func() {
		err := startWriter(&wg, writerQuit, os.Stdout, header, deviceChan)
		mustSucceed("start display writer", err)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Printf("Shutting down...")
		close(scannerQuit)
		close(writerQuit)
	}()

	wg.Wait()
	log.Printf("Scanner and writer have finished.")
}
