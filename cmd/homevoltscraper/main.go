package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"homevoltscraper/internal/scraper"
)

func main() {
	url := flag.String("url", "http://192.168.107.83/battery/", "Homevolt battery status URL")
	chargedSel := flag.String("charged-selector", "", "CSS selector to locate kWh charged text")
	dischargedSel := flag.String("discharged-selector", "", "CSS selector to locate kWh discharged text")
	format := flag.String("format", "text", "Output format: text or json")
	waitSel := flag.String("wait-selector", "", "chromedp: CSS selector to wait for before scraping")
	wait := flag.Duration("wait", 2*time.Second, "chromedp: wait duration before scraping if no selector is provided")
	powerSel := flag.String("power-selector", "", "CSS selector to locate current power text (e.g., 'Power: 290 W')")
	flag.Parse()

	var res scraper.Result
	var err error
	res, err = scraper.FetchAndParseChromedp(scraper.Config{
		URL:                *url,
		ChargedSelector:    *chargedSel,
		DischargedSelector: *dischargedSel,
		PowerSelector:      *powerSel,
		WaitSelector:       *waitSel,
		Wait:               *wait,
	})
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	default:
		fmt.Printf("kWh discharged: %.3f\n", res.KWhDischarged)
		fmt.Printf("kWh charged: %.3f\n", res.KWhCharged)
		fmt.Printf("Power: %.1f W\n", res.PowerW)
	}
}
