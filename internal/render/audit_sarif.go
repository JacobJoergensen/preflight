package render

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"path/filepath"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/model"
)

const (
	sarifSchemaURI = "https://json.schemastore.org/sarif-2.1.0.json"
	sarifVersion   = "2.1.0"
	preflightURI   = "https://github.com/JacobJoergensen/preflight"
)

type SARIFAuditRenderer struct {
	Out         io.Writer
	ToolVersion string
}

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri"`
	Version        string      `json:"version,omitempty"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string         `json:"id"`
	ShortDescription sarifText      `json:"shortDescription"`
	HelpURI          string         `json:"helpUri,omitempty"`
	Properties       sarifRuleProps `json:"properties"`
}

type sarifRuleProps struct {
	SecuritySeverity string   `json:"security-severity"`
	Tags             []string `json:"tags,omitempty"`
}

type sarifResult struct {
	RuleID              string            `json:"ruleId"`
	Level               string            `json:"level"`
	Message             sarifText         `json:"message"`
	Locations           []sarifLocation   `json:"locations"`
	PartialFingerprints map[string]string `json:"partialFingerprints,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifText struct {
	Text string `json:"text"`
}

func (r SARIFAuditRenderer) Render(report result.AuditReport) error {
	rulesByID := make(map[string]sarifRule)

	var ruleOrder []string

	results := make([]sarifResult, 0)

	for _, item := range report.Items {
		for _, finding := range item.Findings {
			id := sarifRuleID(item, finding)
			severity := ecosystem.NormalizeSeverity(finding.Severity)

			if _, ok := rulesByID[id]; !ok {
				rulesByID[id] = sarifRule{
					ID:               id,
					ShortDescription: sarifText{Text: sarifRuleDescription(finding, id)},
					HelpURI:          finding.URL,
					Properties:       sarifRuleProps{SecuritySeverity: securitySeverity(severity), Tags: []string{"security"}},
				}
				ruleOrder = append(ruleOrder, id)
			}

			results = append(results, sarifResult{
				RuleID:              id,
				Level:               sarifLevel(severity),
				Message:             sarifText{Text: sarifMessage(finding)},
				Locations:           []sarifLocation{{PhysicalLocation: sarifPhysicalLocation{ArtifactLocation: sarifArtifactLocation{URI: sarifLocationURI(item)}}}},
				PartialFingerprints: map[string]string{"preflightFindingHash": sarifFingerprint(item, finding, id)},
			})
		}
	}

	rules := make([]sarifRule, 0, len(ruleOrder))

	for _, id := range ruleOrder {
		rules = append(rules, rulesByID[id])
	}

	document := sarifLog{
		Schema:  sarifSchemaURI,
		Version: sarifVersion,
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:           "preflight",
				InformationURI: preflightURI,
				Version:        r.ToolVersion,
				Rules:          rules,
			}},
			Results: results,
		}},
	}

	return encodeJSON(r.Out, document, true)
}

func sarifRuleID(item result.AuditItem, finding model.Finding) string {
	if finding.ID != "" {
		return finding.ID
	}

	if finding.Package != "" {
		return item.ScopeID + ":" + finding.Package
	}

	return item.ScopeID + ":vulnerability"
}

func sarifRuleDescription(finding model.Finding, id string) string {
	if finding.Summary != "" {
		return finding.Summary
	}

	if finding.Package != "" {
		return "Vulnerability in " + finding.Package
	}

	return id
}

func sarifMessage(finding model.Finding) string {
	message := finding.Summary

	if finding.Package != "" {
		pkg := finding.Package

		if finding.Version != "" {
			pkg += "@" + finding.Version
		}

		if message == "" {
			message = pkg
		} else {
			message = pkg + ": " + message
		}
	}

	if message == "" {
		message = finding.ID
	}

	if finding.FixedIn != "" {
		message += " (fixed in " + finding.FixedIn + ")"
	}

	if message == "" {
		return "Vulnerability detected"
	}

	return message
}

func sarifLevel(severity string) string {
	switch severity {
	case "critical", "high":
		return "error"
	case "moderate":
		return "warning"
	default:
		return "note"
	}
}

func securitySeverity(severity string) string {
	switch severity {
	case "critical":
		return "9.8"
	case "high":
		return "8.1"
	case "moderate":
		return "5.5"
	case "low":
		return "2.0"
	default:
		return "0.0"
	}
}

func sarifLocationURI(item result.AuditItem) string {
	manifest := item.Manifest

	if manifest == "" {
		manifest = "."
	}

	if item.Project == "" {
		return manifest
	}

	return filepath.ToSlash(item.Project) + "/" + manifest
}

func sarifFingerprint(item result.AuditItem, finding model.Finding, ruleID string) string {
	seed := strings.Join([]string{item.Project, item.ScopeID, ruleID, finding.Package}, "|")
	sum := sha256.Sum256([]byte(seed))

	return hex.EncodeToString(sum[:])
}
