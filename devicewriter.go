package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
)

type screenWriter struct {
	wg       *sync.WaitGroup
	ptab     table.Writer
	header   table.Row
	quit     chan any
	readPath ingestPath
}

func newWriter(wg *sync.WaitGroup, f *os.File, header table.Row, q chan any, r ingestPath) *screenWriter {
	ptab := table.NewWriter()
	ptab.SetTitle("Bluetooth Devices")
	ptab.SetOutputMirror(f)
	ptab.AppendHeader(header)
	return &screenWriter{
		wg:       wg,
		ptab:     ptab,
		header:   header,
		readPath: r,
		quit:     q,
	}
}

func startWriter(wg *sync.WaitGroup, q chan any, f *os.File, header table.Row, readp ingestPath) error {
	// create a new writer
	w := newWriter(wg, f, header, q, readp)
	go w.execute()
	return nil
}

func (d *screenWriter) execute() {
	for {
		select {
		case <-d.quit:
			d.wg.Done()
			return
		case devices := <-d.readPath:
			d.Write(devices)
		}
	}
}

func (d *screenWriter) Write(devs []devContent) {
	// fmt.Println("writer: writing devices to screen...")
	for _, v := range devs {
		companyName := resolveCompanyIdent(&cmap, v.companyIdent)
		localName := v.localName
		findMyDevice := isFindMyDevice(v.manufacturerData)
		timeSince := time.Since(v.lastSeen)
		d.ptab.AppendRow(table.Row{
			fmt.Sprintf("%v", v.id),
			localName,
			companyName,
			findMyDevice,
			fmt.Sprintf("%.0fs", timeSince.Truncate(time.Second).Seconds()),
		})
	}
	// Set the table style.
	d.ptab.SetStyle(table.StyleColoredBright)
	// clears the screen.
	clearScreen()
	// Render the table.
	d.ptab.Render()
	// Reset the rows in the table.
	d.ptab.ResetRows()
}