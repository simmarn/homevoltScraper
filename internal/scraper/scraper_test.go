package scraper

import (
	"testing"
)

func TestParseHTML_ChargedDischargedPatterns(t *testing.T) {
	html := `<html><body>State: Running Setpoint: 300 W DischargePower: 290 W (290 VA)
	Constraints: 6028 W discharge power available state: 3 alarms: 0
	02.11 kWh charged 8.98 kWh discharged 229.2/231.3/230 V 49.975 Hz ID: INV0</body></html>`

	res, err := ParseHTML(html, Config{URL: "http://example.local/battery/"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.KWhCharged == 0 {
		t.Errorf("expected charged > 0, got %v", res.KWhCharged)
	}
	if res.KWhDischarged == 0 {
		t.Errorf("expected discharged > 0, got %v", res.KWhDischarged)
	}
}
