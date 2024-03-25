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
	ptab.SetTitle("Apple FindMy Devices")
	ptab.SetOutputMirror(f)
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
	termHeight, err := getTerminalHeight()
	if err != nil {
		termHeight = 15
	}
	rowBuff := 5
	// fmt.Println("writer: writing devices to screen...")
	d.ptab.AppendHeader(table.Row{
		fmt.Sprintf("Unique Apple FindMy Devices: %v", len(devs)),
	})
	d.ptab.SetStyle(table.StyleColoredBlackOnCyanWhite)
	d.ptab.AppendSeparator()
	d.ptab.AppendRow(d.header)
	for _, v := range devs[:min(len(devs), termHeight-rowBuff)] {
		if isFindMyDevice(v.manufacturerData) {
			companyName := resolveCompanyIdent(&cmap, v.companyIdent)
			localName := v.localName
			findMyDevice := isFindMyDevice(v.manufacturerData)
			firstSeen := time.Since(v.firstSeen)
			lastSeen := time.Since(v.lastSeen)
			d.ptab.AppendRow(table.Row{
				fmt.Sprintf("%v", v.id),
				localName,
				companyName,
				findMyDevice,
				fmt.Sprintf("%.0fs", firstSeen.Truncate(time.Second).Seconds()),
				fmt.Sprintf("%.0fs", lastSeen.Truncate(time.Second).Seconds()),
				v.timesSeen,
			})
		}
	}
	d.ptab.AppendRow(table.Row{
		"...",
	})
	d.ptab.AppendFooter(table.Row{fmt.Sprintf("Last Updated: %v", time.Now().Format("2006-01-02 15:04:05"))})
	// Set the table style.
	// d.ptab.SetStyle(table.StyleColoredBright)
	// clears the screen.
	clearScreen()
	// // Render the table.
	d.ptab.Render()
	// Reset the rows in the table.
	d.ptab.ResetRows()
	d.ptab.ResetFooters()
	d.ptab.ResetHeaders()
}
