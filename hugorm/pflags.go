package hugorm

import (
	"fmt"
	"github.com/spf13/cast"
	"github.com/spf13/pflag"
	"strings"
)

// FlagValue is an interface that users can implement
// to bind different flags to viper.
type FlagValue interface {
	ExplicitlyGiven() bool
	Name() string
	ValueString() string
	ValueType() string
}

// FlagValueSet is an interface that users can implement
// to bind a set of flags to viper.
type FlagValueSet interface {
	VisitAll(fn func(FlagValue))
}

// pflagValueSet is a wrapper around *pflag.ValueSet
// that implements FlagValueSet.
type pflagValueSet struct {
	flags *pflag.FlagSet
}

// VisitAll iterates over all *pflag.Flag inside the *pflag.FlagSet.
func (p pflagValueSet) VisitAll(fn func(flag FlagValue)) {
	p.flags.VisitAll(func(flag *pflag.Flag) {
		fn(pflagValue{flag})
	})
}

// pflagValue is a wrapper aroung *pflag.flag
// that implements FlagValue
type pflagValue struct {
	flag *pflag.Flag
}

// ExplicitlyGiven returns whether the flag has changes or not.
func (p pflagValue) ExplicitlyGiven() bool {
	return p.flag.Changed
}

// Name returns the name of the flag.
func (p pflagValue) Name() string {
	return p.flag.Name
}

// ValueString returns the value of the flag as a string.
func (p pflagValue) ValueString() string {
	return p.flag.Value.String()
}

// ValueType returns the type of the flag as a string.
func (p pflagValue) ValueType() string {
	return p.flag.Value.Type()
}

//=============================================================================

// BindPFlag binds a specific key to a pflag (as used by cobra).
// Example (where serverCmd is a Cobra instance):
//
//	 serverCmd.Flags().Int("port", 1138, "Port to run Application server on")
//	 Viper.BindPFlag("port", serverCmd.Flags().Lookup("port"))
//
func BindPFlag(key string, flag *pflag.Flag) error { return hg.BindPFlag(key, flag) }

func (h *Hugorm) BindPFlag(key string, flag *pflag.Flag) error {
	if flag == nil {
		return fmt.Errorf("flag for %q is nil", key)
	}
	return h.BindFlagValue(key, pflagValue{flag})
}

// BindPFlags binds a full flag set to the configuration, using each flag's long
// name as the config key.
func BindPFlags(flags *pflag.FlagSet) error { return hg.BindPFlags(flags) }

func (h *Hugorm) BindPFlags(flags *pflag.FlagSet) error {
	return h.BindFlagValues(pflagValueSet{flags})
}

// BindFlagValue binds a specific key to a FlagValue.
func BindFlagValue(key string, flag FlagValue) error { return hg.BindFlagValue(key, flag) }

func (h *Hugorm) BindFlagValue(key string, flag FlagValue) error {
	if flag == nil {
		return fmt.Errorf("flag for %q is nil", key)
	}
	h.pflags[key] = flag

	h.invalidateCache()

	return nil
}

// BindFlagValues binds a full FlagValue set to the configuration, using each flag's long
// name as the config key.
func BindFlagValues(flags FlagValueSet) error { return hg.BindFlagValues(flags) }

func (h *Hugorm) BindFlagValues(flags FlagValueSet) (err error) {
	flags.VisitAll(func(flag FlagValue) {
		if err = h.BindFlagValue(flag.Name(), flag); err != nil {
			return
		}
	})
	return nil
}

//--------------------------------------------------------------------------------------

func castMapFlagToMapInterface(src map[string]FlagValue) map[string]interface{} {
	tgt := map[string]interface{}{}
	for k, v := range src {
		tgt[k] = v
	}
	return tgt
}

func (h *Hugorm) flagBindings2configMap(bindings map[string]FlagValue) (result map[string]interface{}) {

	result = make(map[string]interface{})

	for b, flag := range bindings {
		var val interface{}
		path := strings.Split(b, h.keyDelim)
		if flag.ExplicitlyGiven() {
			switch flag.ValueType() {
			case "int", "int8", "int16", "int32", "int64":
				val = cast.ToInt(flag.ValueString())
			case "bool":
				val = cast.ToBool(flag.ValueString())
			case "stringSlice":
				s := strings.TrimPrefix(flag.ValueString(), "[")
				s = strings.TrimSuffix(s, "]")
				val, _ = readAsCSV(s)
			case "intSlice":
				s := strings.TrimPrefix(flag.ValueString(), "[")
				s = strings.TrimSuffix(s, "]")
				res, _ := readAsCSV(s)
				val = cast.ToIntSlice(res)
			case "stringToString":
				val = stringToStringConv(flag.ValueString())
			default:
				val = flag.ValueString()
			}
			setKeyInMap(result, path, val)
		}
	}
	return
}
