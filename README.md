# HomevoltScraper

A small Go CLI that fetches the Holmevolt/Homevolt battery status page and extracts kWh charged and kWh discharged values.

## Build

```bash
cd /home/martin/git/homevoltScraper
go mod tidy
GO111MODULE=on go build -o bin/homevoltscraper ./cmd/homevoltscraper
```

## Usage

```bash
./bin/homevoltscraper \
  -url "http://192.168.107.83/battery/" \
  -format json
```

Optional flags:
- `-charged-selector` and `-discharged-selector`: CSS selectors if the values are in specific elements.
- `-timeout`: HTTP timeout (e.g. `10s`).
- `-user` / `-pass`: Basic auth if the page is protected.
- `-format`: `text` (default) or `json`.

Examples:

```bash
# Default text output
./bin/homevoltscraper

# JSON output
./bin/homevoltscraper -format json

# Using explicit selectors (adjust to your DOM)
./bin/homevoltscraper -charged-selector ".charged" -discharged-selector ".discharged"

# Headless render (chromedp only)
./bin/homevoltscraper -format json

# Wait until a specific element appears (recommended)
./bin/homevoltscraper -wait-selector "body" -format json

# Or just wait a bit before scraping
./bin/homevoltscraper -wait 3s -format json
```

## Notes
- If selectors are not provided, the scraper falls back to scanning nearby text for keywords like "charged"/"discharged" and reading the closest numeric value.
- If parsing fails, it reports which value couldn't be parsed.
- The parser specifically matches patterns like "2.11 kWh charged" and "8.98 kWh discharged" anywhere in page text.
- chromedp renders the page; use `-wait-selector` (preferred) or `-wait` to ensure data is loaded.
