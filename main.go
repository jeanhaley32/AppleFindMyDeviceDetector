package main

import (
	"log"
	"os"
	"sync"

	"github.com/jedib0t/go-pretty/v6/table"
)

const (
	adManSpecData            = byte(0xFF)   // 0xFF is the AD type for manufacturer specific data.
	appleIdentifier          = byte(0x004C) // 0x004C is the company identifier for Apple.
	findMyNetworkBroadcastID = byte(0x12)   // 0x12 is the broadcast ID for the FindMy network.
)

var (
	cmap   = make(CorpIdentMap)
	header = table.Row{"Device ID", "Name", "Company", "FindMy", "Last Seen"}
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
