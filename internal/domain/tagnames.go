package domain

// TagNames is the list of tag titles attached to a row, in stored order.
type TagNames []string

// MarshalJSON renders a nil list as [] so JSON output never shows null tags.
func (names TagNames) MarshalJSON() ([]byte, error) {
	if names == nil {
		return []byte("[]"), nil
	}

	return MarshalCompactJSON([]string(names))
}
