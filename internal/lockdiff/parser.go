package lockdiff

type Parser interface {
	Ecosystem() string
	Parse(data []byte) (map[string]string, error)
}

var parsers = map[string]Parser{}

func Register(filename string, parser Parser) {
	parsers[filename] = parser
}

func ParserFor(filename string) (Parser, bool) {
	parser, ok := parsers[filename]
	return parser, ok
}

func RegisteredFilenames() []string {
	names := make([]string, 0, len(parsers))

	for name := range parsers {
		names = append(names, name)
	}

	return names
}
