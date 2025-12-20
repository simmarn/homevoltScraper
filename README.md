# HomevoltScraper

A small Go CLI that fetches the Homevolt battery status page and extracts kWh charged, kWh discharged, and current power (W).

## Build

```bash
cd /home/martin/git/homevoltScraper
go mod tidy
go build -o bin/homevoltscraper ./cmd/homevoltscraper
```

### Container (Podman)

```bash
# Build image
podman build -t homevoltscraper .

# Run one-shot publish to MQTT (uses host network to reach local URL)
podman run --rm --network=host homevoltscraper \
	-url "http://<homevolt-host>/battery/" \
	-mqtt-broker "tcp://<mqtt-host>:1883" \
	-interval 0

# Continuous mode every 300s (default)
podman run --rm --network=host homevoltscraper \
	-url "http://<homevolt-host>/battery/" \
	-mqtt-broker "tcp://<mqtt-host>:1883"
```

## Usage

`-url` is required. MQTT flags are optional; without a broker, the JSON payload is printed.

```bash
./bin/homevoltscraper -url "http://<homevolt-host>/battery/" \
	-mqtt-broker "tcp://<mqtt-host>:1883" \
	-mqtt-topic "homevolt/status"
```

Output: JSON.

Examples:

```bash
# Print JSON to stdout
./bin/homevoltscraper -url "http://<homevolt-host>/battery/"

# Publish to MQTT (QoS 0, retain=false; default topic: homevolt/status)
./bin/homevoltscraper -url "http://<homevolt-host>/battery/" \
	-mqtt-broker "tcp://<mqtt-host>:1883"

# Publish to a custom topic with credentials
./bin/homevoltscraper -url "http://<homevolt-host>/battery/" \
	-mqtt-broker "tcp://<mqtt-host>:1883" \
	-mqtt-topic "my/homevolt/topic" \
	-mqtt-user "user" -mqtt-pass "pass"

## Continuous mode
By default, the tool fetches and publishes continuously every 300 seconds.

```bash
# Custom interval (e.g., 60s)
./bin/homevoltscraper -url "http://<homevolt-host>/battery/" \
	-mqtt-broker "tcp://<mqtt-host>:1883" \
	-interval 60s

# One-shot (no loop)
./bin/homevoltscraper -url "http://<homevolt-host>/battery/" \
	-mqtt-broker "tcp://<mqtt-host>:1883" \
	-interval 0

# Continuous printing to stdout (no MQTT)
./bin/homevoltscraper -url "http://<homevolt-host>/battery/" \
	-interval 120s
```
```

## Notes
- The scraper renders the page via headless Chrome (chromedp) and parses text-only; no CSS selectors or extra flags are required.
- If parsing fails, it reports which value couldn't be parsed.
- The parser specifically matches patterns like "2.11 kWh charged" and "8.98 kWh discharged" anywhere in page text.
 - The scraper always waits for the `body` element to be visible before parsing.
 - Power is extracted from patterns like `Power: -290 W`, `ChargePower: 300 W` (negative when charging), and `DischargePower: 300 W` (positive).
 - CLI output swaps charged/discharged to correct the webpage's mislabeled values.
