package scraper

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

type Config struct {
	URL string
}

type Result struct {
	KWhDischarged float64 `json:"kWh_discharged"`
	KWhCharged    float64 `json:"kWh_charged"`
	PowerW        float64 `json:"power_W"`
}

// ParseHTML parses the provided HTML and extracts kWh charged/discharged.
func ParseHTML(html string, cfg Config) (Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return Result{}, err
	}
	return parseDoc(doc, cfg)
}

// ParseText parses plain visible text content for kWh and power values.
func ParseText(text string, cfg Config) (Result, error) {
	var out Result
	low := text
	if vc, vd, ok := extractKWhFromText(low); ok {
		out.KWhCharged = vc
		out.KWhDischarged = vd
	}
	if pv, ok := extractPowerWFromText(low); ok {
		out.PowerW = pv
	}
	if out.KWhCharged == 0 || out.KWhDischarged == 0 {
		// Try nearby matches as a fallback
		if out.KWhCharged == 0 {
			if v, ok := regexFindKWh(low, []string{"charged", "charge"}); ok {
				out.KWhCharged, _ = parseKWh(v)
			} else {
				out.KWhCharged, _ = parseKWh(findValueNear(low, []string{"charged", "charge"}))
			}
		}
		if out.KWhDischarged == 0 {
			if v, ok := regexFindKWh(low, []string{"discharged", "discharge"}); ok {
				out.KWhDischarged, _ = parseKWh(v)
			} else {
				out.KWhDischarged, _ = parseKWh(findValueNear(low, []string{"discharged", "discharge"}))
			}
		}
	}
	parseErrs := []string{}
	if out.KWhCharged == 0 {
		parseErrs = append(parseErrs, "charged")
	}
	if out.KWhDischarged == 0 {
		parseErrs = append(parseErrs, "discharged")
	}
	if len(parseErrs) > 0 {
		return out, errors.New("failed to parse: " + strings.Join(parseErrs, ", "))
	}
	return out, nil
}

// parseDoc contains the core parsing logic operating on a goquery Document.
func parseDoc(doc *goquery.Document, cfg Config) (Result, error) {
	var out Result
	// Consider whole document text for additional signals
	fullText := doc.Text()
	// If either value missing, try direct extraction from text first
	if out.KWhCharged == 0 || out.KWhDischarged == 0 {
		if vc, vd, ok := extractKWhFromText(fullText); ok {
			if out.KWhCharged == 0 {
				out.KWhCharged = vc
			}
			if out.KWhDischarged == 0 {
				out.KWhDischarged = vd
			}
		}
	}
	// If still missing, attempt to locate nearby text via regex/keywords
	var chargedText, dischargedText string
	if out.KWhCharged == 0 {
		if v, ok := regexFindKWh(fullText, []string{"charged", "charge"}); ok {
			chargedText = v
		} else {
			chargedText = findValueNear(fullText, []string{"charged", "charge"})
		}
	}
	if out.KWhDischarged == 0 {
		if v, ok := regexFindKWh(fullText, []string{"discharged", "discharge"}); ok {
			dischargedText = v
		} else {
			dischargedText = findValueNear(fullText, []string{"discharged", "discharge"})
		}
	}

	var parseErrs []string
	if out.KWhCharged == 0 {
		if v, ok := parseKWh(chargedText); ok {
			out.KWhCharged = v
		} else {
			parseErrs = append(parseErrs, "charged")
		}
	}
	if out.KWhDischarged == 0 {
		if v, ok := parseKWh(dischargedText); ok {
			out.KWhDischarged = v
		} else {
			parseErrs = append(parseErrs, "discharged")
		}
	}

	// Power parsing: parse from text using patterns
	if pv, ok := extractPowerWFromText(fullText); ok {
		out.PowerW = pv
	}

	if len(parseErrs) > 0 {
		return out, errors.New("failed to parse: " + strings.Join(parseErrs, ", "))
	}
	return out, nil
}

// FetchAndParseChromedp renders the page via headless Chrome and extracts values.
func FetchAndParseChromedp(cfg Config) (Result, error) {
	var out Result
	// Create a quiet chromedp context to avoid noisy logs to stderr
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoSandbox,
	)...)
	defer cancelAlloc()
	// Suppress chromedp logs (Info/Debug/Error)
	nop := func(string, ...any) {}
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(nop), chromedp.WithDebugf(nop), chromedp.WithErrorf(nop))
	defer cancel()
	// Timeout to avoid hanging.
	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var html string
	var innerText string
	tasks := chromedp.Tasks{chromedp.Navigate(cfg.URL), chromedp.WaitVisible("body", chromedp.ByQuery), chromedp.Sleep(2 * time.Second)}
	tasks = append(tasks, chromedp.OuterHTML("html", &html, chromedp.ByQuery))
	tasks = append(tasks, chromedp.Text("body", &innerText, chromedp.ByQuery))
	if err := chromedp.Run(ctx, tasks); err != nil {
		return out, err
	}

	// Prefer parsing visible text; fallback to HTML if needed
	if res, err := ParseText(innerText, cfg); err == nil {
		return res, nil
	}
	return ParseHTML(html, cfg)
}

// RenderHTMLChromedp navigates and returns the full rendered HTML for separate parsing and tests.
func RenderHTMLChromedp(cfg Config) (string, error) {
	// Create a quiet chromedp context
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoSandbox,
	)...)
	defer cancelAlloc()
	nop := func(string, ...any) {}
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(nop), chromedp.WithDebugf(nop), chromedp.WithErrorf(nop))
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var html string
	tasks := chromedp.Tasks{chromedp.Navigate(cfg.URL), chromedp.WaitVisible("body", chromedp.ByQuery), chromedp.Sleep(2 * time.Second)}
	tasks = append(tasks, chromedp.OuterHTML("html", &html, chromedp.ByQuery))
	if err := chromedp.Run(ctx, tasks); err != nil {
		return "", err
	}
	return html, nil
}

func parseKWh(s string) (float64, bool) {
	// Extract a number optionally followed by kWh
	s = strings.ToLower(s)
	for i := 0; i < len(s); i++ {
		if (s[i] >= '0' && s[i] <= '9') || s[i] == '.' {
			// read number
			j := i
			for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == '.') {
				j++
			}
			valStr := s[i:j]
			// ensure it's a plausible float
			if strings.Count(valStr, ".") <= 1 {
				var v float64
				_, err := fmt.Sscanf(valStr, "%f", &v)
				if err == nil {
					return v, true
				}
			}
		}
	}
	return 0, false
}

// parsePowerW extracts a power value in watts from a short text like "Power: 290 W" or "-300 W".
// (parsePowerW removed: superseded by extractPowerWFromText parsing logic)

// regexFindKWh tries several patterns around provided keywords to locate a numeric kWh value
func regexFindKWh(text string, keywords []string) (string, bool) {
	low := strings.ToLower(text)
	// Build patterns like: (charged|charge)[^\n]*?([0-9]+(?:\.[0-9]+)?)\s*kwh
	kwGroup := strings.Join(keywords, "|")
	patterns := []string{
		"(" + kwGroup + ")[^\n]*?([0-9]+(?:\\.[0-9]+)?)\\s*kwh",
		"([0-9]+(?:\\.[0-9]+)?)\\s*kwh[^\n]*?(" + kwGroup + ")",
		// Without explicit unit, still capture a nearby number
		"(" + kwGroup + ")[^\n]*?([0-9]+(?:\\.[0-9]+)?)",
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if m := re.FindStringSubmatch(low); len(m) >= 3 {
			// Return the matched substring portion containing the number
			return m[0], true
		}
	}
	return "", false
}

func findValueNear(text string, keywords []string) string {
	low := strings.ToLower(text)
	for _, kw := range keywords {
		idx := strings.Index(low, kw)
		if idx >= 0 {
			// take a window around keyword
			start := idx
			if start-64 > 0 {
				start -= 64
			}
			end := idx + 64
			if end > len(text) {
				end = len(text)
			}
			return text[start:end]
		}
	}
	return ""
}

// extractKWhFromText looks for explicit patterns like:
//
//	2.11 kWh charged   and   8.98 kWh discharged
//
// It returns both values if found.
func extractKWhFromText(text string) (charged float64, discharged float64, ok bool) {
	chargedRe := regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*kwh\s*charged`)
	dischargedRe := regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*kwh\s*discharged`)
	if m := chargedRe.FindStringSubmatch(text); len(m) == 2 {
		var cv float64
		if _, err := fmt.Sscanf(m[1], "%f", &cv); err == nil {
			charged = cv
		}
	}
	if m := dischargedRe.FindStringSubmatch(text); len(m) == 2 {
		var dv float64
		if _, err := fmt.Sscanf(m[1], "%f", &dv); err == nil {
			discharged = dv
		}
	}
	return charged, discharged, charged != 0 || discharged != 0
}

// extractPowerWFromText looks for patterns:
// - "Power: -290 W" (negative when charging)
// - "DischargePower: 290 W" (positive)
// - "ChargePower: 290 W" (negative)
func extractPowerWFromText(text string) (float64, bool) {
	low := strings.ToLower(text)
	// Prefer explicit generic Power label first. Support optional colon, unicode minus (U+2212),
	// and flexible spacing before the unit.
	rePower := regexp.MustCompile(`\bpower\b[^0-9\-−]*([\-−]?[0-9]+(?:\.[0-9]+)?)\s*w`)
	if m := rePower.FindStringSubmatch(low); len(m) == 2 {
		var v float64
		num := strings.ReplaceAll(m[1], "−", "-")
		if _, err := fmt.Sscanf(num, "%f", &v); err == nil {
			return v, true
		}
	}
	// ChargePower: respect explicit sign; support unicode minus and optional colon
	reCh := regexp.MustCompile(`(?i)(?:\bcharge\s*power\b|\bchargepower\b)[^0-9\-−]*([\-−]?[0-9]+(?:\.[0-9]+)?)\s*w`)
	if m := reCh.FindStringSubmatch(low); len(m) == 2 {
		var v float64
		num := strings.ReplaceAll(m[1], "−", "-")
		if _, err := fmt.Sscanf(num, "%f", &v); err == nil {
			// If sign not present, v is positive; charging should be negative.
			// However, when ChargePower is explicitly labeled, prefer given sign.
			return v, true
		}
	}
	// DischargePower: respect explicit sign; support unicode minus and optional colon
	reDis := regexp.MustCompile(`(?i)(?:\bdischarge\s*power\b|\bdischargepower\b)[^0-9\-−]*([\-−]?[0-9]+(?:\.[0-9]+)?)\s*w`)
	if m := reDis.FindStringSubmatch(low); len(m) == 2 {
		var v float64
		num := strings.ReplaceAll(m[1], "−", "-")
		if _, err := fmt.Sscanf(num, "%f", &v); err == nil {
			return v, true
		}
	}
	// IdlePower: often shows small values when system is idle; treat as current power.
	reIdle := regexp.MustCompile(`(?i)(?:\bidle\s*power\b|\bidlepower\b)[^0-9\-−]*([\-−]?[0-9]+(?:\.[0-9]+)?)\s*w`)
	if m := reIdle.FindStringSubmatch(low); len(m) == 2 {
		var v float64
		num := strings.ReplaceAll(m[1], "−", "-")
		if _, err := fmt.Sscanf(num, "%f", &v); err == nil {
			return v, true
		}
	}
	// Note: We do not use Setpoint or generic number+W fallbacks; if no labeled power present, fallback is 0 W.
	return 0, false
}
