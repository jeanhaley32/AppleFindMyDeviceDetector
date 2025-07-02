package writer

import (
	"airtagtracker/internal/corp"
	"airtagtracker/internal/device"
	"airtagtracker/pkg/util"
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

type ScreenWriter struct {
	wg       *sync.WaitGroup
	ptab     table.Writer
	header   table.Row
	quit     chan any
	readPath chan device.List
	dc       device.List
	cmap     corp.CorpIdentMap
}

func NewWriter(wg *sync.WaitGroup, f *os.File, header table.Row, q chan any, r chan device.List, cmap corp.CorpIdentMap) *ScreenWriter {
	ptab := table.NewWriter()
	ptab.SetTitle("Apple FindMy Devices")
	ptab.SetOutputMirror(f)
	return &ScreenWriter{
		wg:       wg,
		ptab:     ptab,
		header:   header,
		readPath: r,
		quit:     q,
		cmap:     cmap,
	}
}

func (d *ScreenWriter) Start() {
	d.execute()
}

func (d *ScreenWriter) execute() {
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

func (d *ScreenWriter) Write() {
	termHeight, err := util.GetTerminalHeight()
	if err != nil {
		termHeight = 15
	}
	rowBuff := 5
	// fmt.Println("writer: writing devices to screen...")
	d.ptab.AppendHeader(table.Row{fmt.Sprintf("Unique Apple FindMy Devices: %v Scan Loops: %v", len(d.dc.Devices), d.dc.ScanCount)})
	d.ptab.SetStyle(table.StyleColoredBlackOnCyanWhite)
	d.ptab.AppendSeparator()
	d.ptab.AppendRow(d.header)
	for _, v := range d.dc.Devices[:util.Min(len(d.dc.Devices), termHeight-rowBuff)] {
		PercentSeen := 0
		if d.dc.ScanCount > 0 {
			PercentSeen = v.TimesSeen * 100 / d.dc.ScanCount
		}
		AirTag := ""
		if v.IsAppleAirTag() {
			AirTag = "*"
		}
		registered := ""
		if v.IsRegistered() {
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
				// fmt.Sprintf("...%X", v.AddressString()[len(v.AddressString())-8:]),
				fmt.Sprintf("%v", v.D.Address.String()),
				fmt.Sprintf("%v", d.cmap.Resolve(v.CompanyIdent())),
				fmt.Sprintf("%v: %v", vlist, len(vlist)), //vlist[:min(len(vlist)/2, 4)], len(vlist)
				AirTag,
				registered,
				fmt.Sprintf("%v:%v:%v",
					v.SinceFirstSeen().Round(time.Second),
					v.SinceLastSeen().Round(time.Second),
					v.DetectedFor().Round(time.Second),
				),
				v.TimesSeen,
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
	util.ClearScreen()
	// // Render the table.
	d.ptab.Render()
	// Reset the rows in the table.
	d.ptab.ResetRows()
	d.ptab.ResetFooters()
	d.ptab.ResetHeaders()
}
