package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
)

const (
	adManSpecData            = byte(0xFF)   // 0xFF is the AD type for manufacturer specific data.
	appleIdentifier          = byte(0x004C) // 0x004C is the company identifier for Apple.
	findMyNetworkBroadcastID = byte(0x12)   // 0x12 is the broadcast ID for the FindMy network.
)

var (
	cmap = make(CorpIdentMap)
)

func main() {
	// Load the company identifiers into a map.
	cmap = ingestCorpDevices(companyIdentlocation)
	// create ingestion path
	ingp := make(ingestPath)
	// create a new table writer.
	ptab := table.NewWriter()
	// Set the output to stdout.
	ptab.SetOutputMirror(os.Stdout)
	// Set the table headers.
	ptab.AppendHeader(table.Row{"Device ID", "Name", "Company", "FindMy"})
	// Create a wait group to wait for the scanner to finish. we're not actually doing that in this iteration
	wg := sync.WaitGroup{}
	// start the scanner in a go routine.
	go startBleScanner(&wg, &ingp)
	// listen for devices on the ingestion path.
	for devices := range ingp {
		fmt.Println("Devices found:")
		devices.Range(func(k, v interface{}) bool {
			// iterate over the devices and add them to the table.
			for k, v := range v.(map[uint16]devContent) {
				companyName := resolveCompanyIdent(&cmap, v.companyIdent)
				localName := v.localName
				findMyDevice := isFindMyDevice(v.manufacturerData)
				ptab.AppendRow(table.Row{
					fmt.Sprintf("%x", k),
					localName,
					companyName,
					findMyDevice,
				})
			}
			return true
		})
		// Set the table style.
		ptab.SetStyle(table.StyleRounded)
		// clears the screen.
		clearScreen()
		// Render the table.
		ptab.Render()
		// Reset the rows in the table.
		ptab.ResetRows()
		// Wait x seconds before scanning again.
		time.Sleep(scanWait)
	}
}
