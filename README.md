# HomevoltScraper

A small Go CLI that fetches the Homevolt battery status page and extracts kWh charged, kWh discharged, and current power (W).

## Build

```bash
cd /home/martin/git/homevoltScraper
go mod tidy
go build -o bin/homevoltscraper ./cmd/homevoltscraper
```

## Usage

```bash
./bin/homevoltscraper -url "http://192.168.107.83/battery/"
```

Output: JSON.

Examples:

```bash
# Basic usage (JSON output)
./bin/homevoltscraper -url "http://192.168.107.83/battery/"

# Pretty-print with jq
./bin/homevoltscraper -url "http://192.168.107.83/battery/" | jq
```

## Notes
- The scraper renders the page via headless Chrome (chromedp) and parses text-only; no CSS selectors or extra flags are required.
- If parsing fails, it reports which value couldn't be parsed.
- The parser specifically matches patterns like "2.11 kWh charged" and "8.98 kWh discharged" anywhere in page text.
 - The scraper always waits for the `body` element to be visible before parsing.
 - Power is extracted from patterns like `Power: -290 W`, `ChargePower: 300 W` (negative when charging), and `DischargePower: 300 W` (positive).
 - CLI output swaps charged/discharged to correct the webpage's mislabeled values.
