package domain

import (
	"bytes"
	"encoding/json"
)

type TagNames []string

func (names TagNames) MarshalJSON() ([]byte, error) {
	if names == nil {
		return []byte("[]"), nil
	}

	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode([]string(names)); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(encoded.Bytes(), []byte("\n")), nil
}
