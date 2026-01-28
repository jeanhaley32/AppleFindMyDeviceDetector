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
	header = table.Row{"Dev ID", "Manufacturer", "Manufacturer Data", "AirTag", "AirPods", "Owner Near", "Battery", "First:Last:Delta", "Times Seen", "% Seen"}
)

type screenWriter struct {
	wg         *sync.WaitGroup
	table      table.Writer
	header     table.Row
	quit       chan any
	inputChan  DeviceChannel
	devices    deviceList
}

func newWriter(wg *sync.WaitGroup, output *os.File, header table.Row, quit chan any, input DeviceChannel) *screenWriter {
	tableWriter := table.NewWriter()
	tableWriter.SetTitle("Apple FindMy Devices")
	tableWriter.SetOutputMirror(output)
	return &screenWriter{
		wg:        wg,
		table:     tableWriter,
		header:    header,
		inputChan: input,
		quit:      quit,
	}
}

func startWriter(wg *sync.WaitGroup, quit chan any, output *os.File, header table.Row, input DeviceChannel) error {
	writer := newWriter(wg, output, header, quit, input)
	go writer.execute()
	return nil
}

func (w *screenWriter) execute() {
	for {
		select {
		case <-w.quit:
			w.wg.Done()
			return
		case deviceList := <-w.inputChan:
			w.devices = deviceList
			w.render()
		}
	}
}

// render draws the device table to the terminal.
func (w *screenWriter) render() {
	termWidth, termHeight, err := getTerminalSize()
	if err != nil || termHeight < 10 {
		termHeight = 15
	}
	if err != nil || termWidth < 80 {
		termWidth = 120
	}
	reservedRows := 5

	w.table.AppendHeader(table.Row{fmt.Sprintf("Unique Apple FindMy Devices: %v Scan Loops: %v", len(w.devices.devices), w.devices.scanCount)})
	w.table.SetStyle(table.StyleColoredBlackOnCyanWhite)

	// Prevent word wrapping by setting allowed row length to terminal width
	w.table.SetAllowedRowLength(termWidth)

	// Hide empty columns and suppress trailing spaces
	w.table.SuppressEmptyColumns()
	w.table.SuppressTrailingSpaces()

	w.table.AppendSeparator()
	w.table.AppendRow(w.header)

	maxRows := min(len(w.devices.devices), termHeight-reservedRows)
	for _, dev := range w.devices.devices[:maxRows] {
		percentSeen := 0
		if w.devices.scanCount > 0 {
			percentSeen = dev.timesSeen * 100 / w.devices.scanCount
		}

		isAirTag := ""
		if dev.isAppleAirTag() {
			isAirTag = "*"
		}

		isAirPods := ""
		if dev.isAirPods() {
			isAirPods = "*"
		}

		ownerNearby := ""
		if dev.isOwnerNearby() {
			ownerNearby = "*"
		}

		batteryLevel := dev.getBatteryLevel()

		var hexBytes []string
		for _, payload := range dev.ManufacturerData() {
			if len(payload) > 0 {
				for _, b := range payload {
					hexBytes = append(hexBytes, fmt.Sprintf("%X", b))
				}
			} else {
				w.table.AppendRow(table.Row{"None"})
			}
			w.table.AppendRow(table.Row{
				dev.scanResult.Address.String(),
				getCompanyName(&companyNames, dev.CompanyID()),
				fmt.Sprintf("%v: %v", hexBytes, len(hexBytes)),
				isAirTag,
				isAirPods,
				ownerNearby,
				batteryLevel,
				fmt.Sprintf("%v:%v:%v",
					dev.sinceFirstSeen().Round(time.Second),
					dev.sinceLastSeen().Round(time.Second),
					dev.detectedFor().Round(time.Second),
				),
				dev.TimesSeen(),
				fmt.Sprintf("%v%%", percentSeen),
			})
		}
	}
	w.table.AppendRow(table.Row{
		"...",
	})
	w.table.SetColumnConfigs([]table.ColumnConfig{
		{Number: 3, WidthMax: 40, WidthMaxEnforcer: text.Trim}, // Manufacturer Data - truncate if too long
		{Number: 4, Align: text.AlignCenter},                   // AirTag
		{Number: 5, Align: text.AlignCenter},                   // AirPods
		{Number: 6, Align: text.AlignCenter},                   // Owner Near
		{Number: 7, Align: text.AlignCenter},                   // Battery
	})

	w.table.AppendFooter(table.Row{fmt.Sprintf("Last Updated: %v", time.Now().Format("2006-01-02 15:04:05"))})
	// clears the screen.
	clearTerminal()
	// // Render the table.
	w.table.Render()
	// Reset the rows in the table.
	w.table.ResetRows()
	w.table.ResetFooters()
	w.table.ResetHeaders()
}
