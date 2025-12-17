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
	URL          string
	WaitSelector string
	Wait         time.Duration
}

type Result struct {
	KWhDischarged float64 `json:"kWh_discharged"`
	KWhCharged    float64 `json:"kWh_charged"`
	PowerW        float64 `json:"power_W"`
	Source        string  `json:"source"`
}

// ParseHTML parses the provided HTML and extracts kWh charged/discharged.
func ParseHTML(html string, cfg Config) (Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return Result{}, err
	}
	return parseDoc(doc, cfg)
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
	out.Source = cfg.URL
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
	waitDur := cfg.Wait
	if waitDur <= 0 {
		waitDur = 2 * time.Second
	}
	tasks := chromedp.Tasks{chromedp.Navigate(cfg.URL)}
	if strings.TrimSpace(cfg.WaitSelector) != "" {
		tasks = append(tasks, chromedp.WaitVisible(cfg.WaitSelector, chromedp.ByQuery))
	} else {
		tasks = append(tasks, chromedp.Sleep(waitDur))
	}
	tasks = append(tasks, chromedp.OuterHTML("html", &html, chromedp.ByQuery))
	if err := chromedp.Run(ctx, tasks); err != nil {
		return out, err
	}

	// Parse using the shared HTML parser
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
	waitDur := cfg.Wait
	if waitDur <= 0 {
		waitDur = 2 * time.Second
	}
	tasks := chromedp.Tasks{chromedp.Navigate(cfg.URL)}
	if strings.TrimSpace(cfg.WaitSelector) != "" {
		tasks = append(tasks, chromedp.WaitVisible(cfg.WaitSelector, chromedp.ByQuery))
	} else {
		tasks = append(tasks, chromedp.Sleep(waitDur))
	}
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
func parsePowerW(s string) (float64, bool) {
	if strings.TrimSpace(s) == "" {
		return 0, false
	}
	re := regexp.MustCompile(`(?i)power:\s*(-?[0-9]+(?:\.[0-9]+)?)\s*w`)
	if m := re.FindStringSubmatch(s); len(m) == 2 {
		var v float64
		if _, err := fmt.Sscanf(m[1], "%f", &v); err == nil {
			return v, true
		}
	}
	// Fallback: any number followed by W
	re2 := regexp.MustCompile(`(?i)(-?[0-9]+(?:\.[0-9]+)?)\s*w`)
	if m := re2.FindStringSubmatch(s); len(m) == 2 {
		var v float64
		if _, err := fmt.Sscanf(m[1], "%f", &v); err == nil {
			return v, true
		}
	}
	return 0, false
}

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
	// Prefer explicit generic Power: first, which includes the correct sign when charging/discharging.
	rePower := regexp.MustCompile(`\bpower:\s*(-?[0-9]+(?:\.[0-9]+)?)\s*w`)
	if m := rePower.FindStringSubmatch(low); len(m) == 2 {
		var v float64
		if _, err := fmt.Sscanf(m[1], "%f", &v); err == nil {
			return v, true
		}
	}
	// ChargePower: respect an explicit sign if present; otherwise assume negative (charging)
	reCh := regexp.MustCompile(`(?i)(?:\bcharge\s*power\b|\bchargepower\b)\s*:\s*(-?[0-9]+(?:\.[0-9]+)?)\s*w`)
	if m := reCh.FindStringSubmatch(low); len(m) == 2 {
		var v float64
		if _, err := fmt.Sscanf(m[1], "%f", &v); err == nil {
			// If sign not present, v is positive; charging should be negative.
			// However, when ChargePower is explicitly labeled, prefer given sign.
			return v, true
		}
	}
	// DischargePower: respect explicit sign; otherwise assume positive
	reDis := regexp.MustCompile(`(?i)(?:\bdischarge\s*power\b|\bdischargepower\b)\s*:\s*(-?[0-9]+(?:\.[0-9]+)?)\s*w`)
	if m := reDis.FindStringSubmatch(low); len(m) == 2 {
		var v float64
		if _, err := fmt.Sscanf(m[1], "%f", &v); err == nil {
			return v, true
		}
	}
	return 0, false
}
