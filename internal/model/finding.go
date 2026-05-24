package model

type Finding struct {
	ID       string   // primary advisory id: GHSA-…, RUSTSEC-…, GO-…, PYSEC-…, or CVE-…
	Aliases  []string // other ids for the same advisory (e.g. CVE ids); drives suppression and dedup
	Severity string   // normalized severity: critical, high, moderate, low, or info
	Package  string   // affected package
	Version  string   // installed or affected version, when reported
	FixedIn  string   // first fixed version, when reported
	URL      string   // advisory URL
	Summary  string   // one-line description
}
