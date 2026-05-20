package render

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}

	return "s"
}
