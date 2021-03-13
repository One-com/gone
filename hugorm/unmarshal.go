package hugorm

import (
	"fmt"
	"github.com/mitchellh/mapstructure"
)

// ConfigMarshalError happens when failing to marshal the configuration.
type ConfigMarshalError struct {
	err error
}

// Error returns the formatted configuration error.
func (e ConfigMarshalError) Error() string {
	return fmt.Sprintf("While marshaling config: %s", e.err.Error())
}

// UnmarshalKey takes a single key and unmarshals it into a Struct.
func UnmarshalKey(key string, rawVal interface{}, opts ...DecoderConfigOption) error {
	return hg.UnmarshalKey(key, rawVal, opts...)
}

func (h *Hugorm) UnmarshalKey(key string, rawVal interface{}, opts ...DecoderConfigOption) error {
	return decode(h.Get(key), defaultDecoderConfig(rawVal, opts...))
}

//TODO
//// Unmarshal unmarshals the config into a Struct. Make sure that the tags
//// on the fields of the structure are properly set.
//func Unmarshal(rawVal interface{}, opts ...DecoderConfigOption) error {
//	return hg.Unmarshal(rawVal, opts...)
//}
//
//func (h *Hugorm) Unmarshal(rawVal interface{}, opts ...DecoderConfigOption) error {
//	return decode(v.AllSettings(), defaultDecoderConfig(rawVal, opts...))
//}

// A DecoderConfigOption can be passed to viper.Unmarshal to configure
// mapstructure.DecoderConfig options
type DecoderConfigOption func(*mapstructure.DecoderConfig)

// DecodeHook returns a DecoderConfigOption which overrides the default
// DecoderConfig.DecodeHook value, the default is:
//
//  mapstructure.ComposeDecodeHookFunc(
//		mapstructure.StringToTimeDurationHookFunc(),
//		mapstructure.StringToSliceHookFunc(","),
//	)
func DecodeHook(hook mapstructure.DecodeHookFunc) DecoderConfigOption {
	return func(c *mapstructure.DecoderConfig) {
		c.DecodeHook = hook
	}
}

// defaultDecoderConfig returns default mapsstructure.DecoderConfig with suppot
// of time.Duration values & string slices
func defaultDecoderConfig(output interface{}, opts ...DecoderConfigOption) *mapstructure.DecoderConfig {
	c := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           output,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// A wrapper around mapstructure.Decode that mimics the WeakDecode functionality
func decode(input interface{}, config *mapstructure.DecoderConfig) error {
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}
	return decoder.Decode(input)
}

// TODO
//// UnmarshalExact unmarshals the config into a Struct, erroring if a field is nonexistent
//// in the destination struct.
//func UnmarshalExact(rawVal interface{}, opts ...DecoderConfigOption) error {
//	return hg.UnmarshalExact(rawVal, opts...)
//}
//
//func (h *Hugorm) UnmarshalExact(rawVal interface{}, opts ...DecoderConfigOption) error {
//	config := defaultDecoderConfig(rawVal, opts...)
//	config.ErrorUnused = true
//
//	return decode(v.AllSettings(), config)
//}
