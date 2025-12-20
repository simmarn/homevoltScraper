package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"homevoltscraper/internal/scraper"
)

func main() {
	url := flag.String("url", "", "Homevolt battery status URL (required)")
	flag.Parse()

	if *url == "" {
		fmt.Fprintln(os.Stderr, "error: -url is required")
		flag.Usage()
		os.Exit(2)
	}

	res, err := scraper.FetchAndParseChromedp(scraper.Config{URL: *url})
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Correct mislabeled webpage: always swap charged/discharged in output
	res.KWhCharged, res.KWhDischarged = res.KWhDischarged, res.KWhCharged

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(res); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode json: %v\n", err)
	}

}
