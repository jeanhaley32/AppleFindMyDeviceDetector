package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	cmap = make(CorpIdentMap)
)

func main() {
	cmap = ingestCorpDevices(companyIdentlocation)
	ingp := make(ingestPath)
	wg := sync.WaitGroup{}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Buffered channels prevent goroutine leaks on shutdown
	scannerQuit := make(chan any, 1)
	writerQuit := make(chan any, 1)

	// start the scanner in a go routine.
	wg.Add(2)
	go func() {
		// create a new scanner'
		err := startBleScanner(
			&wg,
			ingp,
			scannerQuit,
		)
		must("Failed to start BlueTooth Scanner", err)
	}()
	go func() {
		// start the writer
		err := startWriter(
			&wg,
			writerQuit,
			os.Stdout,
			header,
			ingp,
		)
		must("Failed to start writer", err)
	}()

	// Handle shutdown signal
	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v. Shutting down gracefully...", sig)
		scannerQuit <- struct{}{}
		writerQuit <- struct{}{}
	}()

	wg.Wait()
	log.Printf("Scanner and writer have finished.")
}
