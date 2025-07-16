package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

var (
	header = table.Row{"Dev ID", "Manufacturer", "Manufacturer Data", "AirTag", "registered", "First:Last:Delta", "Times Seen", "Percent Seen"}
)

type screenWriter struct {
	wg       *sync.WaitGroup
	ptab     table.Writer
	header   table.Row
	quit     chan any
	readPath ingestPath
	dc       deviceList
}

func newWriter(wg *sync.WaitGroup, f *os.File, header table.Row, q chan any, r ingestPath) *screenWriter {
	log.Printf("[DEBUG] Creating new screen writer instance")
	ptab := table.NewWriter()
	ptab.SetTitle("Apple FindMy Devices")
	ptab.SetOutputMirror(f)
	log.Printf("[DEBUG] Screen writer configured with title and output mirror")
	return &screenWriter{
		wg:       wg,
		ptab:     ptab,
		header:   header,
		readPath: r,
		quit:     q,
	}
}

func startWriter(wg *sync.WaitGroup, q chan any, f *os.File, header table.Row, readp ingestPath) error {
	log.Printf("[DEBUG] Starting device writer with header: %v", header)
	// create a new writer
	w := newWriter(wg, f, header, q, readp)
	log.Printf("[DEBUG] Launching writer execution goroutine")
	go w.execute()
	return nil
}

func (d *screenWriter) execute() {
	log.Printf("[DEBUG] Device writer execution started")
	d.Write()
	log.Printf("[DEBUG] Initial write completed, entering main writer loop")
	for {
		select {
		case <-d.quit:
			log.Printf("[DEBUG] Device writer received quit signal")
			d.wg.Done()
			return
		case devices := <-d.readPath:
			log.Printf("[DEBUG] Received device list update - %d devices, scan count: %d",
				len(devices.devices), devices.scanCount)
			d.dc = devices
			d.Write()
			log.Printf("[DEBUG] Screen update completed")
		}
	}
}

func (d *screenWriter) Write() {
	log.Printf("[DEBUG] Starting screen write operation")
	termHeight, err := getTerminalHeight()
	if err != nil {
		log.Printf("[DEBUG] Failed to get terminal height, using default: %v", err)
		termHeight = 15
	} else {
		log.Printf("[DEBUG] Terminal height detected: %d", termHeight)
	}
	rowBuff := 5
	maxDisplayRows := termHeight - rowBuff
	log.Printf("[DEBUG] Will display maximum %d device rows", maxDisplayRows)

	// fmt.Println("writer: writing devices to screen...")
	header := table.Row{fmt.Sprintf("Unique Apple FindMy Devices: %v Scan Loops: %v", len(d.dc.devices), d.dc.scanCount)}
	d.ptab.AppendHeader(header)
	d.ptab.SetStyle(table.StyleColoredBlackOnCyanWhite)
	d.ptab.AppendSeparator()
	d.ptab.AppendRow(d.header)
	log.Printf("[DEBUG] Table header and styling configured")

	deviceCount := len(d.dc.devices)
	displayCount := min(deviceCount, maxDisplayRows)
	log.Printf("[DEBUG] Processing %d devices for display (out of %d total)", displayCount, deviceCount)

	for i, v := range d.dc.devices[:displayCount] {
		log.Printf("[DEBUG] Processing device %d: %s", i+1, v.AddressString())

		PercentSeen := 0
		if d.dc.scanCount > 0 {
			PercentSeen = v.timesSeen * 100 / d.dc.scanCount
		}
		log.Printf("[DEBUG] Device %s stats - times seen: %d, scan count: %d, percent: %d%%",
			v.AddressString(), v.timesSeen, d.dc.scanCount, PercentSeen)

		AirTag := ""
		if v.isAppleAirTag() {
			AirTag = "*"
			log.Printf("[DEBUG] Device %s marked as AirTag", v.AddressString())
		}
		registered := ""
		if v.isRegistered() {
			registered = "*"
			log.Printf("[DEBUG] Device %s marked as registered", v.AddressString())
		}

		var vlist []string
		for _, b := range v.ManufacturerData() {
			if len(b) > 0 {
				for _, i := range b {
					vlist = append(vlist, fmt.Sprintf("%X", i))
				}
			} else {
				log.Printf("[DEBUG] Device %s has empty manufacturer data", v.AddressString())
				d.ptab.AppendRow(table.Row{"None"})
			}
			log.Printf("[DEBUG] Device %s manufacturer data processed - %d bytes", v.AddressString(), len(vlist))

			d.ptab.AppendRow(table.Row{
				// fmt.Sprintf("...%X", v.AddressString()[len(v.AddressString())-8:]),
				fmt.Sprintf("%v", v.d.Address.String()),
				fmt.Sprintf("%v", resolveCompanyIdent(&cmap, v.CompanyIdent())),
				fmt.Sprintf("%v: %v", vlist, len(vlist)), //vlist[:min(len(vlist)/2, 4)], len(vlist)
				AirTag,
				registered,
				fmt.Sprintf("%v:%v:%v",
					v.sinceFirstSeen().Round(time.Second),
					v.sinceLastSeen().Round(time.Second),
					v.detectedFor().Round(time.Second),
				),
				v.TimesSeen(),
				fmt.Sprintf("%v%%", PercentSeen),
			})
		}
	}

	d.ptab.AppendRow(table.Row{
		"...",
	})
	d.ptab.SetColumnConfigs([]table.ColumnConfig{
		{Number: 4, Align: text.AlignCenter},
		{Number: 5, Align: text.AlignCenter},
	})
	log.Printf("[DEBUG] Table column configuration applied")

	currentTime := time.Now().Format("2006-01-02 15:04:05")
	d.ptab.AppendFooter(table.Row{fmt.Sprintf("Last Updated: %v", currentTime)})
	log.Printf("[DEBUG] Table footer added with timestamp: %s", currentTime)

	// clears the screen.
	log.Printf("[DEBUG] Clearing screen before rendering table")
	clearScreen()
	// // Render the table.
	log.Printf("[DEBUG] Rendering table to screen")
	d.ptab.Render()
	// Reset the rows in the table.
	log.Printf("[DEBUG] Resetting table for next update")
	d.ptab.ResetRows()
	d.ptab.ResetFooters()
	d.ptab.ResetHeaders()
	log.Printf("[DEBUG] Screen write operation completed")
}
