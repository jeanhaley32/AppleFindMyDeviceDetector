package main

import (
	"airtagtracker/internal/corp"
	"airtagtracker/internal/device"
	"airtagtracker/internal/scanner"
	"airtagtracker/internal/writer"
	"airtagtracker/pkg/util"
	"log"
	"os"
	"sync"

	"github.com/jedib0t/go-pretty/v6/table"
	"tinygo.org/x/bluetooth"
)

func main() {
	cmap, err := corp.IngestCorpDevices("company_identifiers.yaml")
	util.Must("Failed to ingest company identifiers", err)

	ingp := make(chan device.List)
	wg := sync.WaitGroup{}

	adapter := bluetooth.DefaultAdapter
	quitChan := make(chan any)

	scanner := scanner.NewScanner(&wg, adapter, &sync.Map{}, quitChan)
	writer := writer.NewWriter(&wg, os.Stdout, table.Row{"Dev ID", "Manufacturer", "Manufacturer Data", "AirTag", "registered", "First:Last:Delta", "Times Seen", "Percent Seen"}, quitChan, ingp, cmap)

	wg.Add(2)
	go func() {
		defer wg.Done()
		err := scanner.Start(ingp)
		util.Must("Failed to start bluetooth scanner", err)
	}()

	go func() {
		defer wg.Done()
		writer.Start()
	}()

	wg.Wait()
	log.Printf("Scanner and writer have finished.")
}
