package adapter

import (
	"context"
	"encoding/json"
	"math"
	"strings"
)

func (r RustModule) Audit(ctx context.Context, deps Dependencies) AuditResult {
	if !deps.Loader.LoadCargoConfig().HasManifest {
		return AuditResult{Skipped: true, SkipReason: "no Cargo.toml"}
	}

	return executeAudit(ctx, deps.Loader.WorkDir, auditCommand{
		Name:            "cargo-audit",
		Display:         "cargo audit",
		Args:            []string{"audit", "--json"},
		ParseCounts:     parseCargoAuditCounts,
		ToolMissingHint: "cargo-audit not found on PATH (install: cargo install cargo-audit)",
	})
}

func parseCargoAuditCounts(jsonText string) map[string]int {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" {
		return nil
	}

	var report struct {
		Vulnerabilities struct {
			List []struct {
				Advisory struct {
					Informational string `json:"informational"`
					CVSS          string `json:"cvss"`
				} `json:"advisory"`
			} `json:"list"`
		} `json:"vulnerabilities"`
	}

	if err := json.Unmarshal([]byte(jsonText), &report); err != nil {
		return nil
	}

	counts := make(map[string]int)

	for _, vuln := range report.Vulnerabilities.List {
		counts[advisorySeverity(vuln.Advisory.Informational, vuln.Advisory.CVSS)]++
	}

	if len(counts) == 0 {
		return nil
	}

	return counts
}

func advisorySeverity(informational, cvss string) string {
	if informational != "" || cvss == "" {
		return "info"
	}

	score, ok := cvssBaseScore(cvss)

	if !ok {
		return "info"
	}

	switch {
	case score >= 9.0:
		return "critical"
	case score >= 7.0:
		return "high"
	case score >= 4.0:
		return "moderate"
	case score > 0:
		return "low"
	default:
		return "info"
	}
}

var (
	cvssAttackVector                = map[string]float64{"N": 0.85, "A": 0.62, "L": 0.55, "P": 0.2}
	cvssAttackComplexity            = map[string]float64{"L": 0.77, "H": 0.44}
	cvssUserInteraction             = map[string]float64{"N": 0.85, "R": 0.62}
	cvssImpact                      = map[string]float64{"N": 0, "L": 0.22, "H": 0.56}
	cvssPrivilegesRequiredUnchanged = map[string]float64{"N": 0.85, "L": 0.62, "H": 0.27}
	cvssPrivilegesRequiredChanged   = map[string]float64{"N": 0.85, "L": 0.68, "H": 0.50}
)

func cvssBaseScore(vector string) (float64, bool) {
	metrics, ok := parseCVSSVector(vector)

	if !ok {
		return 0, false
	}

	avVal, ok := cvssAttackVector[metrics["AV"]]

	if !ok {
		return 0, false
	}

	acVal, ok := cvssAttackComplexity[metrics["AC"]]

	if !ok {
		return 0, false
	}

	uiVal, ok := cvssUserInteraction[metrics["UI"]]

	if !ok {
		return 0, false
	}

	scope := metrics["S"]

	var prVal float64

	if scope == "C" {
		prVal, ok = cvssPrivilegesRequiredChanged[metrics["PR"]]
	} else {
		prVal, ok = cvssPrivilegesRequiredUnchanged[metrics["PR"]]
	}

	if !ok {
		return 0, false
	}

	cVal, ok := cvssImpact[metrics["C"]]

	if !ok {
		return 0, false
	}

	iVal, ok := cvssImpact[metrics["I"]]

	if !ok {
		return 0, false
	}

	aVal, ok := cvssImpact[metrics["A"]]

	if !ok {
		return 0, false
	}

	iss := 1 - ((1 - cVal) * (1 - iVal) * (1 - aVal))

	var impact float64

	if scope == "C" {
		impact = 7.52*(iss-0.029) - 3.25*math.Pow(iss-0.02, 15)
	} else {
		impact = 6.42 * iss
	}

	if impact <= 0 {
		return 0, true
	}

	exploitability := 8.22 * avVal * acVal * prVal * uiVal

	var baseScore float64

	if scope == "C" {
		baseScore = math.Min(1.08*(impact+exploitability), 10)
	} else {
		baseScore = math.Min(impact+exploitability, 10)
	}

	return cvssRoundUp(baseScore), true
}

func parseCVSSVector(vector string) (map[string]string, bool) {
	parts := strings.Split(vector, "/")

	if len(parts) < 2 {
		return nil, false
	}

	if parts[0] != "CVSS:3.0" && parts[0] != "CVSS:3.1" {
		return nil, false
	}

	metrics := make(map[string]string)

	for _, part := range parts[1:] {
		key, value, found := strings.Cut(part, ":")

		if !found {
			continue
		}

		metrics[key] = value
	}

	return metrics, true
}

func cvssRoundUp(value float64) float64 {
	intInput := int(math.Round(value * 100000))

	if intInput%10000 == 0 {
		return float64(intInput) / 100000.0
	}

	return float64(intInput/10000+1) / 10.0
}
