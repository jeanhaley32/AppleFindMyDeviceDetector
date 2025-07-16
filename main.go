package main

import (
	"fmt"
	"log"
	"os"
	"sync"
)

var (
	cmap = make(CorpIdentMap)
)

func main() {
	log.Printf("[DEBUG] Starting AppleFindMyDeviceDetector application")

	log.Printf("[DEBUG] Initializing company identifier map from %s", companyIdentlocation)
	cmap = ingestCorpDevices(companyIdentlocation)
	log.Printf("[DEBUG] Successfully loaded company identifier map with %d entries", len(cmap))

	ingp := make(ingestPath)
	wg := sync.WaitGroup{}
	log.Printf("[DEBUG] Created ingest path channel and wait group")

	// start the scanner in a go routine.
	wg.Add(2)
	log.Printf("[DEBUG] Added 2 workers to wait group (scanner and writer)")

	go func() {
		// create a new scanner'
		fmt.Println("Starting BlueTooth Scanner")
		log.Printf("[DEBUG] Launching BLE scanner goroutine")
		err := startBleScanner(
			&wg,
			ingp,
			make(chan any),
		)
		must("Failed to start BlueTooth Scanner", err)
		log.Printf("[DEBUG] BLE scanner initialization completed")
	}()

	go func() {
		// start the writer
		fmt.Println("Starting Writer")
		log.Printf("[DEBUG] Launching device writer goroutine")
		err := startWriter(
			&wg, make(chan any),
			os.Stdout,
			header,
			ingp,
		)
		must("Failed to start writer", err)
		log.Printf("[DEBUG] Device writer initialization completed")
	}()

	log.Printf("[DEBUG] Waiting for all workers to complete...")
	wg.Wait()
	log.Printf("[DEBUG] All workers have finished, application shutting down")
	log.Printf("Scanner and writer have finished.")
}
