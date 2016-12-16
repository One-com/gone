package jconf

import (
	"encoding/json"
	"errors"
)

type SubConfig struct {
	json.RawMessage
	Parsed interface{}
}

var ErrEmptySubConfig = errors.New("Attempt to parse into with a nil Subconfig")

func (m *SubConfig) ParseInto(i interface{}) (err error) {
	if m == nil {
		err = ErrEmptySubConfig
		return
	}
	switch t := i.(type) {
	case *interface{}:
		err = json.Unmarshal(m.RawMessage, t)
		if err == nil {
			m.Parsed = *t
		}
	default:
		err = json.Unmarshal(m.RawMessage, t)
		if err == nil {
			m.Parsed = t
		}
	}
	return
}

func (m *SubConfig) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(m.Parsed)
	if err == nil {
		return b, nil
	}
	return []byte("ERROR: " + err.Error()), nil
}
