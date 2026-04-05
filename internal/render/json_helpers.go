package render

import (
	"encoding/json"
	"errors"
	"io"
)

func encodeJSON(out io.Writer, v any, pretty bool) error {
	if out == nil {
		return errors.New("json renderer: nil writer")
	}

	encoder := json.NewEncoder(out)

	if pretty {
		encoder.SetIndent("", "  ")
	}

	return encoder.Encode(v)
}
