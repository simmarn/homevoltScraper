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
	URL                string
	ChargedSelector    string
	DischargedSelector string
	WaitSelector       string
	Wait               time.Duration
}

type Result struct {
	KWhCharged    float64 `json:"kWh_charged"`
	KWhDischarged float64 `json:"kWh_discharged"`
	Source        string  `json:"source"`
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

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return out, err
	}

	chargedText := selectText(doc, cfg.ChargedSelector)
	dischargedText := selectText(doc, cfg.DischargedSelector)
	if chargedText == "" || dischargedText == "" {
		fullText := doc.Text()
		if vc, vd, ok := extractKWhFromText(fullText); ok {
			out.KWhCharged = vc
			out.KWhDischarged = vd
			out.Source = cfg.URL
			return out, nil
		}
		if v, ok := regexFindKWh(fullText, []string{"charged", "charge"}); ok {
			chargedText = v
		} else {
			chargedText = findValueNear(fullText, []string{"charged", "charge"})
		}
		if v, ok := regexFindKWh(fullText, []string{"discharged", "discharge"}); ok {
			dischargedText = v
		} else {
			dischargedText = findValueNear(fullText, []string{"discharged", "discharge"})
		}
	}
	var parseErrs []string
	if v, ok := parseKWh(chargedText); ok {
		out.KWhCharged = v
	} else {
		parseErrs = append(parseErrs, "charged")
	}
	if v, ok := parseKWh(dischargedText); ok {
		out.KWhDischarged = v
	} else {
		parseErrs = append(parseErrs, "discharged")
	}
	if len(parseErrs) > 0 {
		return out, errors.New("failed to parse: " + strings.Join(parseErrs, ", "))
	}
	out.Source = cfg.URL
	return out, nil
}

func selectText(doc *goquery.Document, sel string) string {
	if strings.TrimSpace(sel) == "" {
		return ""
	}
	var b strings.Builder
	doc.Find(sel).Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			if b.Len() > 0 {
				b.WriteString(" ")
			}
			b.WriteString(text)
		}
	})
	return b.String()
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
