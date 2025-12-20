package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"homevoltscraper/internal/scraper"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	url := flag.String("url", "", "Homevolt battery status URL (required)")
	broker := flag.String("mqtt-broker", "", "MQTT broker URL (e.g., tcp://host:1883)")
	topic := flag.String("mqtt-topic", "homevolt/status", "MQTT topic to publish to (default: homevolt/status)")
	mqttUser := flag.String("mqtt-user", "", "MQTT username (optional)")
	mqttPass := flag.String("mqtt-pass", "", "MQTT password (optional)")
	flag.Parse()

	if *url == "" {
		fmt.Fprintln(os.Stderr, "error: -url is required")
		flag.Usage()
		os.Exit(2)
	}
	// MQTT flags are optional; if no broker is provided, print payload to stdout.

	res, err := scraper.FetchAndParseChromedp(scraper.Config{URL: *url})
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Correct mislabeled webpage: always swap charged/discharged in output
	res.KWhCharged, res.KWhDischarged = res.KWhDischarged, res.KWhCharged

	// Prepare JSON payload
	payload, err := json.Marshal(res)
	if err != nil {
		log.Fatalf("failed to encode json: %v", err)
	}

	if *broker == "" {
		// No broker: print JSON payload
		os.Stdout.Write(payload)
		os.Stdout.Write([]byte("\n"))
		return
	}
	// Configure and connect MQTT client
	clientID := fmt.Sprintf("homevoltscraper-%d", time.Now().UnixNano())
	opts := mqtt.NewClientOptions().AddBroker(*broker).SetClientID(clientID)
	if *mqttUser != "" {
		opts.SetUsername(*mqttUser)
	}
	if *mqttPass != "" {
		opts.SetPassword(*mqttPass)
	}
	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("mqtt connect error: %v", token.Error())
	}
	defer c.Disconnect(250)

	// Publish payload (QoS=0, retain=false)
	pub := c.Publish(*topic, 0, false, payload)
	pub.Wait()
	if pub.Error() != nil {
		log.Fatalf("mqtt publish error: %v", pub.Error())
	}

}
