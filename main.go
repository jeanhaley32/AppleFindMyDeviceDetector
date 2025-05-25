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
	var err error
	cmap, err = ingestCorpDevices(companyIdentlocation)
	if err != nil {
		log.Fatalf("Failed to load company identifiers: %v", err)
	}
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
		if err != nil {
			log.Fatalf("Failed to start BlueTooth Scanner: %v", err)
		}
	}()
	go func() {
		// start the writer
		err := startWriter(
			&wg, make(chan any),
			os.Stdout,
			header,
			ingp,
			cmap,
		)
		if err != nil {
			log.Fatalf("Failed to start writer: %v", err)
		}
	}()
	wg.Wait()
	log.Printf("Scanner and writer have finished.")
}
