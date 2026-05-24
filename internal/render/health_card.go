package render

import (
	"fmt"
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

	DepOptionalSuccess  []model.Message `json:"dependencyOptionalSuccesses,omitempty"`
	DepOptionalWarnings []model.Message `json:"dependencyOptionalWarnings,omitempty"`
	DepOptionalInfo     []model.Message `json:"dependencyOptionalInfo,omitempty"`

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

	for _, msg := range item.Successes() {
		if !msg.Nested {
			card.Toolchain = append(card.Toolchain, msg.Text)
			continue
		}

		switch {
		case msg.Optional && msg.Info:
			card.DepOptionalInfo = append(card.DepOptionalInfo, msg)
		case msg.Optional:
			card.DepOptionalSuccess = append(card.DepOptionalSuccess, msg)
		case msg.Dev:
			card.DepDevSuccess = append(card.DepDevSuccess, msg)
		default:
			card.DepSuccess = append(card.DepSuccess, msg)
		}
	}

	for _, msg := range item.Warnings() {
		if msg.Nested {
			switch {
			case msg.Optional:
				card.DepOptionalWarnings = append(card.DepOptionalWarnings, msg)
			case msg.Dev:
				card.DepDevWarnings = append(card.DepDevWarnings, msg)
			default:
				card.DepWarnings = append(card.DepWarnings, msg)
			}
		} else {
			card.FlatWarnings = append(card.FlatWarnings, msg)
		}
	}

	for _, msg := range item.Errors() {
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
	if len(item.Errors()) == 0 {
		return nil
	}

	blockers := make([]string, 0, len(item.Errors()))

	for _, msg := range item.Errors() {
		text := strings.TrimSpace(msg.Text)

		if text != "" {
			blockers = append(blockers, text)
		}
	}

	return blockers
}

func buildSummary(item result.CheckItem, card *HealthCard) string {
	if card.Status == HealthOK {
		return ""
	}

	depErr := len(card.DepErrors) + len(card.DepDevErrors)
	depWarn := len(card.DepWarnings) + len(card.DepDevWarnings) + len(card.DepOptionalWarnings)
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
		parts = append(parts, fmt.Sprintf("%d configuration or environment error%s", flatErr, pluralSuffix(flatErr)))
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

func toolchainMismatchHint(item result.CheckItem) bool {
	for _, msg := range item.Warnings() {
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
	if len(item.Errors()) > 0 {
		return HealthFail
	}

	if len(item.Warnings()) > 0 {
		return HealthWarn
	}

	return HealthOK
}

func extractSuggestedActions(item result.CheckItem) []string {
	seen := make(map[string]struct{})
	var actions []string

	add := func(cmd string) {
		if cmd == "" {
			return
		}

		if _, ok := seen[cmd]; ok {
			return
		}

		seen[cmd] = struct{}{}
		actions = append(actions, cmd)
	}

	// Buckets are tried in priority order; add() dedups, so earlier buckets win.
	buckets := []struct {
		messages []model.Message
		match    func(model.Message) bool
	}{
		{item.Errors(), func(m model.Message) bool { return !m.Nested }},
		{item.Errors(), func(m model.Message) bool { return m.Nested && !m.Dev }},
		{item.Errors(), func(m model.Message) bool { return m.Nested && m.Dev }},
		{item.Warnings(), func(m model.Message) bool { return !m.Nested }},
		{item.Warnings(), func(m model.Message) bool { return m.Nested && !m.Dev }},
		{item.Warnings(), func(m model.Message) bool { return m.Nested && m.Dev }},
	}

	for _, bucket := range buckets {
		for _, msg := range bucket.messages {
			if !bucket.match(msg) {
				continue
			}

			for _, line := range extractRunCommands(msg.Text) {
				add(line)
			}
		}
	}

	return actions
}

func extractRunCommands(text string) []string {
	var commands []string

	for _, marker := range []string{"Run `", "run `"} {
		for remaining := text; strings.Contains(remaining, marker); {
			i := strings.Index(remaining, marker)
			remaining = remaining[i+len(marker):]
			end := strings.Index(remaining, "`")

			if end <= 0 {
				break
			}

			cmd := strings.TrimSpace(remaining[:end])

			if cmd != "" {
				commands = append(commands, cmd)
			}

			remaining = remaining[end+1:]
		}
	}

	return commands
}
