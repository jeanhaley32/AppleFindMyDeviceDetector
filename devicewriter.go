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
	dc       deviceList
	corpMap  CorpIdentMap
}

func newWriter(wg *sync.WaitGroup, f *os.File, header table.Row, q chan any, r ingestPath, corpMap CorpIdentMap) *screenWriter {
	ptab := table.NewWriter()
	ptab.SetTitle("Apple FindMy Devices")
	ptab.SetOutputMirror(f)
	return &screenWriter{
		wg:       wg,
		ptab:     ptab,
		header:   header,
		readPath: r,
		quit:     q,
		corpMap:  corpMap,
	}
}

func startWriter(wg *sync.WaitGroup, q chan any, f *os.File, header table.Row, readp ingestPath, corpMap CorpIdentMap) error {
	// create a new writer
	w := newWriter(wg, f, header, q, readp, corpMap)
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
	"log"
	"os"
	"sync"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (d *screenWriter) setupTable(title string) {
	d.ptab.AppendHeader(table.Row{title})
	d.ptab.SetStyle(table.StyleColoredBlackOnCyanWhite)
	d.ptab.AppendSeparator()
	d.ptab.AppendRow(d.header)
}

func (d *screenWriter) prepareTableRows(devices deviceList, termHeight int) []table.Row {
	rows := []table.Row{}
	rowBuff := 5 // as in original code
	// Ensure termHeight is at least rowBuff to prevent panic with negative slice bounds
	if termHeight < rowBuff {
		termHeight = rowBuff // Or some other sensible minimum
	}

	for _, v := range devices.devices[:min(len(devices.devices), termHeight-rowBuff)] {
		PercentSeen := 0
		if devices.scanCount > 0 {
			PercentSeen = v.timesSeen * 100 / devices.scanCount
		}
		AirTag := ""
		if v.isAppleAirTag() {
			AirTag = "*"
		}
		registered := ""
		if v.isRegistered() {
			registered = "*"
		}

		if len(v.ManufacturerData()) == 0 {
			rows = append(rows, table.Row{
				fmt.Sprintf("%v", v.d.Address.String()),
				fmt.Sprintf("%v", resolveCompanyIdent(&d.corpMap, v.CompanyIdent())),
				"None",
				AirTag,
				registered,
				fmt.Sprintf("%v:%v:%v", v.sinceFirstSeen().Round(time.Second), v.sinceLastSeen().Round(time.Second), v.detectedFor().Round(time.Second)),
				v.TimesSeen(),
				fmt.Sprintf("%v%%", PercentSeen),
			})
		} else {
			// This loop preserves the original behavior where each manufacturer data entry (if multiple exist)
			// for a single device results in a new row in the table.
			for _, b := range v.ManufacturerData() {
				var vlist []string
				if len(b) > 0 {
					for _, i := range b {
						vlist = append(vlist, fmt.Sprintf("%X", i))
					}
				} else {
					// Original code behavior: if a specific manufacturer data entry `b` is empty,
					// it appends a row with "None" AND then appends the actual data row with an empty vlist.
					// This interpretation is based on d.ptab.AppendRow(table.Row{"None"}) being called,
					// and then the main data row append happening outside this else.
					// To replicate, we add "None" to vlist if b is empty.
					// However, the original code `d.ptab.AppendRow(table.Row{"None"})` was a separate row.
					// This current refactoring aims to create a slice of rows to be appended later.
					// The original code's logic here is:
					// if len(b) > 0 { process b } else { d.ptab.AppendRow(table.Row{"None"}) }
					// d.ptab.AppendRow(table.Row{ normal device data with vlist from above })
					// This means if len(b) == 0, an extra "None" row is added *before* the actual device data row (with empty vlist).
					// This refactoring will slightly differ by not adding that separate "None" row,
					// but representing the empty manufacturer data within the device's row itself.
					// For a closer match to the original's peculiar "extra row" behavior, one might need to append
					// table.Row{"None"} directly to `rows` here, but that would be out of context for this device.
					// Let's stick to the provided snippet's interpretation for `prepareTableRows`, which makes more sense.
					// If `b` is empty, `vlist` will be empty, and `fmt.Sprintf("%v: %v", vlist, len(vlist))` will show `[]: 0`.
				}

				rows = append(rows, table.Row{
					fmt.Sprintf("%v", v.d.Address.String()),
					fmt.Sprintf("%v", resolveCompanyIdent(&d.corpMap, v.CompanyIdent())),
					fmt.Sprintf("%v: %v", vlist, len(vlist)),
					AirTag,
					registered,
					fmt.Sprintf("%v:%v:%v", v.sinceFirstSeen().Round(time.Second), v.sinceLastSeen().Round(time.Second), v.detectedFor().Round(time.Second)),
					v.TimesSeen(),
					fmt.Sprintf("%v%%", PercentSeen),
				})
			}
		}
	}
	return rows
}

func (d *screenWriter) finalizeTable() {
	d.ptab.AppendRow(table.Row{
		"...",
	})
	d.ptab.SetColumnConfigs([]table.ColumnConfig{
		{Number: 4, Align: text.AlignCenter},
		{Number: 5, Align: text.AlignCenter},
	})
	d.ptab.AppendFooter(table.Row{fmt.Sprintf("Last Updated: %v", time.Now().Format("2006-01-02 15:04:05"))})
}

func (d *screenWriter) renderAndReset() {
	clearScreen()
	d.ptab.Render()
	d.ptab.ResetRows()
	d.ptab.ResetFooters()
	d.ptab.ResetHeaders()
}

func (d *screenWriter) Write() {
	termHeight, err := getTerminalHeight()
	if err != nil {
		log.Printf("devicewriter: failed to get terminal height: %v, using default %d", err, 15)
		termHeight = 15
	}

	title := fmt.Sprintf("Unique Apple FindMy Devices: %v Scan Loops: %v", len(d.dc.devices), d.dc.scanCount)
	d.setupTable(title)

	tableRows := d.prepareTableRows(d.dc, termHeight)
	for _, row := range tableRows {
		d.ptab.AppendRow(row)
	}

	d.finalizeTable()
	d.renderAndReset()
}
