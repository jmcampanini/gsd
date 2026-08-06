package domain

import (
	"bytes"
	"encoding/json"
)

// json.Marshal always HTML-escapes; an encoder is the only way to opt out.
func MarshalCompactJSON(value any) ([]byte, error) {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(encoded.Bytes(), []byte("\n")), nil
}
