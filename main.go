package main

import (
	"log"
	"os"
	"sync"
)

var (
	cmap = make(CorpIdentMap)
)

func main() {
	cmap = ingestCorpDevices(companyIdentlocation)
	ingp := make(ingestPath)
	wg := sync.WaitGroup{}
	// start the scanner in a go routine.
	wg.Add(2)
	go func() {
		// create a new scanner'
		err := startBleScanner(
			&wg,
			ingp,
			make(chan any),
		)
		must("Failed to start BlueTooth Scanner", err)
	}()
	go func() {
		// start the writer
		err := startWriter(
			&wg, make(chan any),
			os.Stdout,
			header,
			ingp,
		)
		must("Failed to start writer", err)
	}()
	wg.Wait()
	log.Printf("Scanner and writer have finished.")
}
