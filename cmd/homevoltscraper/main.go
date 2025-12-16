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
	// chromedp is always used; no HTTP path
	waitSel := flag.String("wait-selector", "", "chromedp: CSS selector to wait for before scraping")
	wait := flag.Duration("wait", 2*time.Second, "chromedp: wait duration before scraping if no selector is provided")
	flag.Parse()

	// Local file mode removed to simplify CLI and avoid dead code.

	var res scraper.Result
	var err error
	res, err = scraper.FetchAndParseChromedp(scraper.Config{
		URL:                *url,
		ChargedSelector:    *chargedSel,
		DischargedSelector: *dischargedSel,
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
		fmt.Printf("kWh charged: %.3f\n", res.KWhCharged)
		fmt.Printf("kWh discharged: %.3f\n", res.KWhDischarged)
	}
}
