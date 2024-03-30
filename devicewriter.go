package main

import (
	"fmt"
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
	dc       DevContentList
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
			d.dc = devices
			d.Write()
		}
	}
}

func (d *screenWriter) Write() {
	termHeight, err := getTerminalHeight()
	if err != nil {
		termHeight = 15
	}
	rowBuff := 5
	// fmt.Println("writer: writing devices to screen...")
	d.ptab.AppendHeader(table.Row{fmt.Sprintf("Unique Apple FindMy Devices: %v Scan Loops: %v", len(d.dc.devContent), d.dc.scanCount)})
	d.ptab.SetStyle(table.StyleColoredBlackOnCyanWhite)
	d.ptab.AppendSeparator()
	d.ptab.AppendRow(d.header)
	for _, v := range d.dc.devContent[:min(len(d.dc.devContent), termHeight-rowBuff)] {
		PercentSeen := 0
		if d.dc.scanCount > 0 {
			PercentSeen = v.timesSeen * 100 / d.dc.scanCount
		}
		AirTag := ""
		if v.isAppleAirTag() {
			AirTag = "*"
		}
		registered := ""
		if v.isRegistered() {
			registered = "*"
		}
		var vlist []string
		for _, b := range v.ManufacturerData() {
			if len(b) > 0 {
				for _, i := range b {
					vlist = append(vlist, fmt.Sprintf("%X", i))
				}
			} else {
				d.ptab.AppendRow(table.Row{"None"})
			}
			d.ptab.AppendRow(table.Row{
				fmt.Sprintf("...%X", v.AddressString()[len(v.AddressString())-8:]),
				fmt.Sprintf("%v", resolveCompanyIdent(&cmap, v.CompanyIdent())),
				fmt.Sprintf("%v...: %v", vlist[:min(len(vlist)/2, 4)], len(vlist)),
				AirTag,
				registered,
				fmt.Sprintf("%v:%v:%v",
					time.Since(v.FirstSeen()).Round(time.Second),
					time.Since(v.LastSeen()).Round(time.Second),
					v.lastSeen.Sub(v.FirstSeen()).Round(time.Second),
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

	d.ptab.AppendFooter(table.Row{fmt.Sprintf("Last Updated: %v", time.Now().Format("2006-01-02 15:04:05"))})
	// clears the screen.
	clearScreen()
	// // Render the table.
	d.ptab.Render()
	// Reset the rows in the table.
	d.ptab.ResetRows()
	d.ptab.ResetFooters()
	d.ptab.ResetHeaders()
}
