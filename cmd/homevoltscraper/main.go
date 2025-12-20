package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"
	"syscall"

	"homevoltscraper/internal/scraper"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	url := flag.String("url", "", "Homevolt battery status URL (required)")
	broker := flag.String("mqtt-broker", "", "MQTT broker URL (e.g., tcp://host:1883)")
	topic := flag.String("mqtt-topic", "homevolt/status", "MQTT topic to publish to (default: homevolt/status)")
	haPrefix := flag.String("ha-prefix", "homeassistant", "Home Assistant MQTT discovery prefix (default: homeassistant)")
	mqttUser := flag.String("mqtt-user", "", "MQTT username (optional)")
	mqttPass := flag.String("mqtt-pass", "", "MQTT password (optional)")
	interval := flag.Duration("interval", 300*time.Second, "Fetch/publish interval (default: 300s). Set to 0 for one-shot.")
	flag.Parse()

	if *url == "" {
		fmt.Fprintln(os.Stderr, "error: -url is required")
		flag.Usage()
		os.Exit(2)
	}
	// MQTT flags are optional; if no broker is provided, we will print payload to stdout.

	// Configure and connect MQTT client if broker is set
	clientID := fmt.Sprintf("homevoltscraper-%d", time.Now().UnixNano())
	var c mqtt.Client
	if *broker != "" {
		opts := mqtt.NewClientOptions().AddBroker(*broker).SetClientID(clientID)
		if *mqttUser != "" {
			opts.SetUsername(*mqttUser)
		}
		if *mqttPass != "" {
			opts.SetPassword(*mqttPass)
		}
		c = mqtt.NewClient(opts)
		if token := c.Connect(); token.Wait() && token.Error() != nil {
			log.Fatalf("mqtt connect error: %v", token.Error())
		}
		defer c.Disconnect(250)

		// Publish retained Home Assistant discovery configs on startup
		publishHAConfig := func(uniqueID, name, deviceClass, stateClass, unit, valueTemplate string) {
			cfg := map[string]any{
				"name":                 name,
				"unique_id":            uniqueID,
				"state_topic":          *topic,
				"unit_of_measurement":  unit,
				"device_class":         deviceClass,
				"state_class":          stateClass,
				"value_template":       valueTemplate,
				"device": map[string]any{
					"identifiers": []string{"homevolt"},
					"name":        "Homevolt",
				},
			}
			payload, err := json.Marshal(cfg)
			if err != nil {
				log.Printf("ha config encode error: %v", err)
				return
			}
			cfgTopic := fmt.Sprintf("%s/sensor/%s/config", *haPrefix, uniqueID)
			t := c.Publish(cfgTopic, 0, true, payload)
			t.Wait()
			if t.Error() != nil {
				log.Printf("ha config publish error (%s): %v", cfgTopic, t.Error())
			}
		}
		// Energy sensors (kWh)
		publishHAConfig("homevolt_total_charged", "Homevolt Total Charged", "energy", "total_increasing", "kWh", "{{ value_json.kWh_charged }}")
		publishHAConfig("homevolt_total_discharged", "Homevolt Total Discharged", "energy", "total_increasing", "kWh", "{{ value_json.kWh_discharged }}")
		// Power sensor (W)
		publishHAConfig("homevolt_current_power", "Homevolt Current Power", "power", "measurement", "W", "{{ value_json.power_W }}")
	}

	// Helper to fetch and output/publish once
	doOnce := func() {
		res, err := scraper.FetchAndParseChromedp(scraper.Config{URL: *url})
		if err != nil {
			log.Printf("fetch error: %v", err)
			return
		}
		res.KWhCharged, res.KWhDischarged = res.KWhDischarged, res.KWhCharged
		payload, err := json.Marshal(res)
		if err != nil {
			log.Printf("encode error: %v", err)
			return
		}
		if *broker == "" {
			os.Stdout.Write(payload)
			os.Stdout.Write([]byte("\n"))
			return
		}
		pub := c.Publish(*topic, 0, false, payload)
		pub.Wait()
		if pub.Error() != nil {
			log.Printf("mqtt publish error: %v", pub.Error())
		}
	}

	// One-shot mode
	if *interval <= 0 {
		doOnce()
		return
	}

	// Continuous mode with signal handling
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	// Run immediately once, then on each tick
	doOnce()
	for {
		select {
		case <-ticker.C:
			doOnce()
		case s := <-sigc:
			log.Printf("received signal %v, exiting", s)
			return
		}
	}

}
