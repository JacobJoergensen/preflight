package render

import (
	"fmt"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/model"
)

type HealthStatus string

const (
	HealthOK   HealthStatus = "ok"
	HealthWarn HealthStatus = "warn"
	HealthFail HealthStatus = "fail"
	HealthSkip HealthStatus = "skip"
)

type HealthCard struct {
	ScopeID       string       `json:"scopeId"`
	ScopeDisplay  string       `json:"scopeDisplay"`
	Status        HealthStatus `json:"status"`
	ElapsedMillis int64        `json:"elapsedMillis,omitempty"`

	Toolchain []string `json:"toolchain,omitempty"`
	Signals   []string `json:"signals,omitempty"`

	FlatWarnings []model.Message `json:"flatWarnings,omitempty"`
	FlatErrors   []model.Message `json:"flatErrors,omitempty"`

	DepSuccess  []model.Message `json:"dependencySuccesses,omitempty"`
	DepWarnings []model.Message `json:"dependencyWarnings,omitempty"`
	DepErrors   []model.Message `json:"dependencyErrors,omitempty"`

	DepDevSuccess  []model.Message `json:"dependencyDevSuccesses,omitempty"`
	DepDevWarnings []model.Message `json:"dependencyDevWarnings,omitempty"`
	DepDevErrors   []model.Message `json:"dependencyDevErrors,omitempty"`

	SuggestedActions []string `json:"suggestedActions,omitempty"`
	Blockers         []string `json:"blockers,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	PrimaryNextStep  string   `json:"primaryNextStep,omitempty"`
}

func BuildHealthCard(item result.CheckItem) HealthCard {
	card := HealthCard{
		ScopeID:       item.ScopeID,
		ScopeDisplay:  item.ScopeDisplay,
		ElapsedMillis: item.ElapsedMillis,
		Status:        healthStatusFromItem(item),
	}

	card.Signals = append(card.Signals, item.ProjectSignals...)

	for _, msg := range item.Successes {
		switch {
		case msg.Nested:
			if msg.Dev {
				card.DepDevSuccess = append(card.DepDevSuccess, msg)
			} else {
				card.DepSuccess = append(card.DepSuccess, msg)
			}
		case isProjectSignalLine(msg.Text):
			// Structured ProjectSignals from the engine supersede legacy adapter "*found:" echoes
			if len(item.ProjectSignals) > 0 {
				continue
			}

			card.Signals = appendUniqueSignal(card.Signals, msg.Text)
		default:
			card.Toolchain = append(card.Toolchain, msg.Text)
		}
	}

	for _, msg := range item.Warnings {
		if msg.Nested {
			if msg.Dev {
				card.DepDevWarnings = append(card.DepDevWarnings, msg)
			} else {
				card.DepWarnings = append(card.DepWarnings, msg)
			}
		} else {
			card.FlatWarnings = append(card.FlatWarnings, msg)
		}
	}

	for _, msg := range item.Errors {
		if msg.Nested {
			if msg.Dev {
				card.DepDevErrors = append(card.DepDevErrors, msg)
			} else {
				card.DepErrors = append(card.DepErrors, msg)
			}
		} else {
			card.FlatErrors = append(card.FlatErrors, msg)
		}
	}

	card.SuggestedActions = extractSuggestedActions(item)
	card.Blockers = deriveBlockers(item)
	card.Summary = buildSummary(item, &card)
	card.PrimaryNextStep = buildPrimaryNextStep(item, &card)

	return card
}

func deriveBlockers(item result.CheckItem) []string {
	if len(item.Errors) == 0 {
		return nil
	}

	blockers := make([]string, 0, len(item.Errors))

	for _, msg := range item.Errors {
		text := strings.TrimSpace(msg.Text)

		if text != "" {
			blockers = append(blockers, text)
		}
	}

	return blockers
}

func appendUniqueSignal(signals []string, line string) []string {
	if slices.Contains(signals, line) {
		return signals
	}

	return append(signals, line)
}

func buildSummary(item result.CheckItem, card *HealthCard) string {
	if card.Status == HealthOK {
		return ""
	}

	depErr := len(card.DepErrors) + len(card.DepDevErrors)
	depWarn := len(card.DepWarnings) + len(card.DepDevWarnings)
	flatErr := len(card.FlatErrors)
	flatWarn := len(card.FlatWarnings)

	if card.Status == HealthWarn {
		if depWarn > 0 && depErr == 0 && flatErr == 0 {
			return fmt.Sprintf("%d dependency warning(s) — review before continuing.", depWarn)
		}

		if flatWarn > 0 && depErr == 0 && depWarn == 0 {
			if toolchainMismatchHint(item) {
				return "Toolchain or engine version does not match project requirements."
			}

			return fmt.Sprintf("%d warning(s) — review before continuing.", flatWarn)
		}

		return fmt.Sprintf("%d warning(s) — review before continuing.", flatWarn+depWarn)
	}

	// fail
	var parts []string

	if depErr > 0 {
		parts = append(parts, dependencySummaryPhrase(depErr))
	}

	if flatErr > 0 {
		parts = append(parts, fmt.Sprintf("%d configuration or environment error%s", flatErr, pluralS(flatErr)))
	}

	if len(parts) == 0 {
		return "Check failed."
	}

	return strings.Join(parts, "; ") + "."
}

func dependencySummaryPhrase(count int) string {
	if count == 1 {
		return "1 missing or invalid dependency"
	}

	return fmt.Sprintf("%d missing or invalid dependencies", count)
}

func pluralS(count int) string {
	if count == 1 {
		return ""
	}

	return "s"
}

func toolchainMismatchHint(item result.CheckItem) bool {
	for _, msg := range item.Warnings {
		if msg.Nested {
			continue
		}

		text := strings.ToLower(msg.Text)

		if strings.Contains(text, "required") || strings.Contains(text, "engine") || strings.Contains(text, "version") {
			return true
		}
	}

	return false
}

func buildPrimaryNextStep(item result.CheckItem, card *HealthCard) string {
	if card.Status == HealthOK {
		return ""
	}

	if len(card.SuggestedActions) > 0 {
		return card.SuggestedActions[0]
	}

	if item.FixPMHint != "" {
		return "preflight fix --pm=" + item.FixPMHint
	}

	return ""
}

func healthStatusFromItem(item result.CheckItem) HealthStatus {
	if len(item.Errors) > 0 {
		return HealthFail
	}

	if len(item.Warnings) > 0 {
		return HealthWarn
	}

	return HealthOK
}

func isProjectSignalLine(text string) bool {
	trimmed := strings.TrimSpace(text)

	if trimmed == "" {
		return false
	}

	if strings.HasSuffix(trimmed, "found:") {
		return true
	}

	return strings.Contains(trimmed, ".json found") || strings.Contains(trimmed, "go.mod found")
}

func extractSuggestedActions(item result.CheckItem) []string {
	seen := make(map[string]struct{})
	var actions []string

	add := func(command string) {
		if command == "" {
			return
		}

		if _, ok := seen[command]; ok {
			return
		}

		seen[command] = struct{}{}
		actions = append(actions, command)
	}

	tryExtract := func(msg model.Message) {
		for _, line := range extractRunCommands(msg.Text) {
			add(line)
		}
	}

	for _, msg := range item.Errors {
		if !msg.Nested {
			tryExtract(msg)
		}
	}

	for _, msg := range item.Errors {
		if msg.Nested && !msg.Dev {
			tryExtract(msg)
		}
	}

	for _, msg := range item.Errors {
		if msg.Nested && msg.Dev {
			tryExtract(msg)
		}
	}

	for _, msg := range item.Warnings {
		if !msg.Nested {
			tryExtract(msg)
		}
	}

	for _, msg := range item.Warnings {
		if msg.Nested && !msg.Dev {
			tryExtract(msg)
		}
	}

	for _, msg := range item.Warnings {
		if msg.Nested && msg.Dev {
			tryExtract(msg)
		}
	}

	return actions
}

func extractRunCommands(text string) []string {
	var commands []string

	for _, marker := range []string{"Run `", "run `"} {
		for remaining := text; strings.Contains(remaining, marker); {
			index := strings.Index(remaining, marker)
			remaining = remaining[index+len(marker):]
			end := strings.Index(remaining, "`")

			if end <= 0 {
				break
			}

			command := strings.TrimSpace(remaining[:end])

			if command != "" {
				commands = append(commands, command)
			}

			remaining = remaining[end+1:]
		}
	}

	return commands
}
