package render

import (
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

func renderByProject[P any, T any](
	ow *terminal.OutputWriter,
	projects []P,
	items []T,
	projectKey func(P) string,
	itemProject func(T) string,
	renderHeader func(*terminal.OutputWriter, P),
	renderItem func(*terminal.OutputWriter, T),
) {
	if len(projects) == 0 {
		for _, item := range items {
			renderItem(ow, item)
		}

		return
	}

	itemsByProject := make(map[string][]T, len(projects))

	for _, item := range items {
		key := itemProject(item)
		itemsByProject[key] = append(itemsByProject[key], item)
	}

	rendered := false

	for _, project := range projects {
		group := itemsByProject[projectKey(project)]

		if len(group) == 0 {
			continue
		}

		if rendered {
			ow.PrintNewLines(1)
		}

		renderHeader(ow, project)

		for _, item := range group {
			renderItem(ow, item)
		}

		rendered = true
	}
}
