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
- `-format`: `text` (default) or `json`.
- `-wait-selector`: CSS selector to wait for (chromedp).
- `-wait`: duration to wait before scraping if no selector is provided.

Examples:

```bash
# Default text output
./bin/homevoltscraper

# JSON output
./bin/homevoltscraper -format json

# Headless render (chromedp)
./bin/homevoltscraper -format json

# Wait until a specific element appears (recommended)
./bin/homevoltscraper -wait-selector "body" -format json

# Or just wait a bit before scraping
./bin/homevoltscraper -wait 3s -format json
```

## Notes
- The scraper renders the page via headless Chrome (chromedp) and parses text-only; no CSS selectors are required.
- If parsing fails, it reports which value couldn't be parsed.
- The parser specifically matches patterns like "2.11 kWh charged" and "8.98 kWh discharged" anywhere in page text.
- chromedp renders the page; use `-wait-selector` (preferred) or `-wait` to ensure data is loaded.
 - Power is extracted from patterns like `Power: -290 W`, `ChargePower: 300 W` (negative when charging), and `DischargePower: 300 W` (positive).
 - CLI output swaps charged/discharged to correct the webpage's mislabeled values.
