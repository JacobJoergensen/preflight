package adapter

import "strings"

var eolVersions = map[string]map[string]bool{
	"php": {
		"5.6": true,
		"7.0": true, "7.1": true, "7.2": true, "7.3": true, "7.4": true,
		"8.0": true, "8.1": true,
	},
	"node": {
		"10": true, "11": true, "12": true, "13": true, "14": true, "15": true,
		"16": true, "17": true, "18": true, "19": true, "21": true, "23": true,
	},
	"go": {
		"1.12": true, "1.13": true, "1.14": true, "1.15": true,
		"1.16": true, "1.17": true, "1.18": true, "1.19": true,
		"1.20": true, "1.21": true, "1.22": true, "1.23": true,
		"1.24": true,
	},
}

func isEOL(runtime, versionPrefix string) bool {
	versions, exists := eolVersions[strings.ToLower(runtime)]

	if !exists {
		return false
	}

	return versions[versionPrefix]
}
