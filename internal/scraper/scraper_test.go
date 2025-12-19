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
	if res.PowerW <= 0 {
		t.Errorf("expected positive discharge power, got %v", res.PowerW)
	}
}

func TestParseHTML_NegativeChargePower(t *testing.T) {
	html := `<html><body>
	State: Running
	Power: -300 W
	01.00 kWh charged 0.50 kWh discharged
	</body></html>`

	res, err := ParseHTML(html, Config{URL: "http://example.local/battery/"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.PowerW >= 0 {
		t.Errorf("expected negative charging power, got %v", res.PowerW)
	}
	if res.KWhCharged == 0 {
		t.Errorf("expected charged > 0, got %v", res.KWhCharged)
	}
}

func TestParseHTML_ChargePowerIsNegative(t *testing.T) {
	html := `<html><body>
	State: Running
	ChargePower: -300 W
	01.00 kWh charged 0.50 kWh discharged
	</body></html>`

	res, err := ParseHTML(html, Config{URL: "http://example.local/battery/"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.PowerW >= 0 {
		t.Errorf("expected negative power for ChargePower, got %v", res.PowerW)
	}
}

func TestParseHTML_DischargePowerIsPositive(t *testing.T) {
	html := `<html><body>
	State: Running
	DischargePower: 300 W
	00.20 kWh charged 1.00 kWh discharged
	</body></html>`

	res, err := ParseHTML(html, Config{URL: "http://example.local/battery/"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.PowerW <= 0 {
		t.Errorf("expected positive power for DischargePower, got %v", res.PowerW)
	}
}

func TestParseHTML_IdlePowerFallback(t *testing.T) {
	html := `<html><body>
	State: RunningSetpoint: 0 W IdlePower: -5 W (5 VA)Constraints: -6028 W charge power availablestate: 3 alarms: 0
	13.59 kWh charged24.28 kWh discharged232.7/229.9/228.9 V 49.989 HzID: INV
	</body></html>`

	res, err := ParseHTML(html, Config{URL: "http://example.local/battery/"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.PowerW != -5 {
		t.Errorf("expected -5 W from IdlePower, got %v", res.PowerW)
	}
}

func TestParseHTML_FallbackZeroNoPowerLabels(t *testing.T) {
	html := `<html><body>
	State: Running Constraints: 6028 W discharge power available state: 3 alarms: 0
	01.23 kWh charged 4.56 kWh discharged 229.0/231.0/230.0 V 50.000 Hz ID: INV0
	</body></html>`

	res, err := ParseHTML(html, Config{URL: "http://example.local/battery/"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.PowerW != 0 {
		t.Errorf("expected fallback 0 W, got %v", res.PowerW)
	}
}
