package domain

import "encoding/json"

type TagNames []string

func (names TagNames) MarshalJSON() ([]byte, error) {
	if names == nil {
		return []byte("[]"), nil
	}

	return json.Marshal([]string(names))
}
