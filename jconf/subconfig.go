package jconf

import (
	"encoding/json"
	"errors"
)

// SubConfig is an interface type which can passed sub module code.
// It contains the raw JSON data for the module config and can
// be asked to parse that JSON into a module specific Config object.
// The SubConfig will remember the parsed result so any later
// Marshal'ing of the full Config will include the sub module Config
// without the higher level code/config knowing about its structure.
type SubConfig interface {
	ParseInto(interface{}) error
	MarshalJSON() ([]byte, error)
}

type subConfig struct {
	json.RawMessage
	Parsed interface{}
}

// OptionalSubConfig implements the SubConfig interface and silently
// ignores being passed nil JSON values.
// Users of OptionalSubConfig will have to check the resulting Config
// object passed to ParseInto for being nil after it returns
type OptionalSubConfig subConfig

// MandatorySubConfig implements the SubConfig interface which will
// return ErrEmptySubConfig if there is no JSON config. This can help
// catch mistakes in the config.
type MandatorySubConfig subConfig

// ErrEmptySubConfig is returned my ParseInto on MandatorySubConfig
// if there is no JSON data.
var ErrEmptySubConfig = errors.New("Missing mandatory SubConfig")

// DefaultSubConfig returns a placeholder value which can be assigned to
// any field which is an OptionalSubConfig to indicate that if the value
// is missing it should be assumed to be an empty JSON object ("{}") and
// passed into any ParseInto() call on the SubConfig.
// This allows for default values to be specified for entire subtrees of
// the JSON
func DefaultSubConfig() *OptionalSubConfig {
	return &OptionalSubConfig{RawMessage: []byte("{}")}
}

func parse(j json.RawMessage, i interface{}) (o interface{}, err error) {

	switch t := i.(type) {
	case *interface{}:
		err = json.Unmarshal(j, t)
		if err == nil {
			o = *t
		}
	default:
		err = json.Unmarshal(j, t)
		if err == nil {
			o = t
		}
	}
	return
}

// ParseInto on will Unmarshal the SubConfig JSON data into the provided interface
// If called on a nil value (no JSON data) will return ErrEmptySubConfig
func (m *MandatorySubConfig) ParseInto(i interface{}) (err error) {
	if m == nil {
		return ErrEmptySubConfig
	}
	m.Parsed, err = parse(m.RawMessage, i)
	return
}

// ParseInto on will Unmarshal the SubConfig JSON data into the provided interface
// If called on a nil value (no JSON data) will do nothing.
func (m *OptionalSubConfig) ParseInto(i interface{}) (err error) {
	if m == nil {
		return
	}
	m.Parsed, err = parse(m.RawMessage, i)
	return
}

func (m *subConfig) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(m.Parsed)
	if err == nil {
		return b, nil
	}
	return []byte("ERROR: " + err.Error()), nil
}

// MarshalJSON allows the subconfig to be serialized
func (m *MandatorySubConfig) MarshalJSON() ([]byte, error) {
	return (*subConfig)(m).MarshalJSON()
}

// MarshalJSON allows the subconfig to be serialized
func (m *OptionalSubConfig) MarshalJSON() ([]byte, error) {
	return (*subConfig)(m).MarshalJSON()
}
