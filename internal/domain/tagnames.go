package domain

type TagNames []string

func (names TagNames) MarshalJSON() ([]byte, error) {
	if names == nil {
		return []byte("[]"), nil
	}

	return MarshalCompactJSON([]string(names))
}
