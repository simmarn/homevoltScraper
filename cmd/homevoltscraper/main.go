package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"homevoltscraper/internal/scraper"
)

func main() {
	url := flag.String("url", "http://192.168.107.83/battery/", "Homevolt battery status URL")
	chargedSel := flag.String("charged-selector", "", "CSS selector to locate kWh charged text")
	dischargedSel := flag.String("discharged-selector", "", "CSS selector to locate kWh discharged text")
	format := flag.String("format", "text", "Output format: text or json")
	useChromedp := flag.Bool("chromedp", true, "Use headless Chrome to render JS before scraping")
	timeout := flag.Duration("timeout", 5*time.Second, "HTTP client timeout")
	user := flag.String("user", "", "HTTP basic auth username (optional)")
	pass := flag.String("pass", "", "HTTP basic auth password (optional)")
	waitSel := flag.String("wait-selector", "", "chromedp: CSS selector to wait for before scraping")
	wait := flag.Duration("wait", 2*time.Second, "chromedp: wait duration before scraping if no selector is provided")
	flag.Parse()

	client := &http.Client{Timeout: *timeout}

	// Allow reading from a local HTML file by passing file://path
	if u := *url; len(u) > 7 && u[:7] == "file://" {
		// Read local file content and simulate a fetch via goquery
		p := u[7:]
		f, err := os.Open(filepath.Clean(p))
		if err != nil {
			log.Fatalf("open file: %v", err)
		}
		defer f.Close()
		// Minimal inline parse path using scraper internals would require refactor; for now, use HTTP path.
		// Suggest setting a simple local HTTP server to serve the HTML when needed.
	}

	var res scraper.Result
	var err error
	if *useChromedp {
		res, err = scraper.FetchAndParseChromedp(scraper.Config{
			URL:                *url,
			ChargedSelector:    *chargedSel,
			DischargedSelector: *dischargedSel,
			User:               *user,
			Pass:               *pass,
			WaitSelector:       *waitSel,
			Wait:               *wait,
		})
	} else {
		res, err = scraper.FetchAndParse(client, scraper.Config{
			URL:                *url,
			ChargedSelector:    *chargedSel,
			DischargedSelector: *dischargedSel,
			User:               *user,
			Pass:               *pass,
			WaitSelector:       *waitSel,
			Wait:               *wait,
		})
	}
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
