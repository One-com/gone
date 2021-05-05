package hugorm

//NOFLAGYET import (
//NOFLAGYET 	"fmt"
//NOFLAGYET 	"github.com/spf13/cast"
//NOFLAGYET 	"strings"
//NOFLAGYET )
//NOFLAGYET
//NOFLAGYET // FlagValue is an interface that users can implement
//NOFLAGYET // to bind different flags to viper.
//NOFLAGYET type FlagValue interface {
//NOFLAGYET 	Name() string
//NOFLAGYET 	ValueString() string
//NOFLAGYET 	ValueType() string
//NOFLAGYET }
//NOFLAGYET
//NOFLAGYET //type FlagValueWithExplicit interface {
//NOFLAGYET //	FlagValue
//NOFLAGYET //	ExplicitlyGiven() bool
//NOFLAGYET //}
//NOFLAGYET
//NOFLAGYET // FlagValueSet is an interface that users can implement
//NOFLAGYET // to bind a set of flags to viper.
//NOFLAGYET type FlagValueSet interface {
//NOFLAGYET 	VisitAll(fn func(FlagValue))
//NOFLAGYET }
//NOFLAGYET
//NOFLAGYET //=============================================================================
//NOFLAGYET
//NOFLAGYET // BindFlagValue binds a specific key to a FlagValue.
//NOFLAGYET func BindFlagValue(key string, flag FlagValue) error { return hg.BindFlagValue(key, flag) }
//NOFLAGYET
//NOFLAGYET func (h *Hugorm) BindFlagValue(key string, flag FlagValue) error {
//NOFLAGYET 	if flag == nil {
//NOFLAGYET 		return fmt.Errorf("flag for %q is nil", key)
//NOFLAGYET 	}
//NOFLAGYET 	h.flags[key] = flag
//NOFLAGYET
//NOFLAGYET 	h.invalidateCache()
//NOFLAGYET
//NOFLAGYET 	return nil
//NOFLAGYET }
//NOFLAGYET
//NOFLAGYET // BindFlagValues binds a full FlagValue set to the configuration, using each flag's long
//NOFLAGYET // name as the config key.
//NOFLAGYET func BindFlagValues(flags FlagValueSet) error { return hg.BindFlagValues(flags) }
//NOFLAGYET
//NOFLAGYET func (h *Hugorm) BindFlagValues(flags FlagValueSet) (err error) {
//NOFLAGYET 	flags.VisitAll(func(flag FlagValue) {
//NOFLAGYET 		if err = h.BindFlagValue(flag.Name(), flag); err != nil {
//NOFLAGYET 			return
//NOFLAGYET 		}
//NOFLAGYET 	})
//NOFLAGYET 	return nil
//NOFLAGYET }
//NOFLAGYET
//NOFLAGYET //--------------------------------------------------------------------------------------
//NOFLAGYET
//NOFLAGYET func castMapFlagToMapInterface(src map[string]FlagValue) map[string]interface{} {
//NOFLAGYET 	tgt := map[string]interface{}{}
//NOFLAGYET 	for k, v := range src {
//NOFLAGYET 		tgt[k] = v
//NOFLAGYET 	}
//NOFLAGYET 	return tgt
//NOFLAGYET }
//NOFLAGYET
//NOFLAGYET func (h *Hugorm) flagBindings2configMap(bindings map[string]FlagValue) (result map[string]interface{}) {
//NOFLAGYET
//NOFLAGYET 	result = make(map[string]interface{})
//NOFLAGYET
//NOFLAGYET 	for b, flag := range bindings {
//NOFLAGYET 		var val interface{}
//NOFLAGYET 		path := strings.Split(b, h.keyDelim)
//NOFLAGYET 		if true {
//NOFLAGYET 			//if flag.ExplicitlyGiven() {
//NOFLAGYET 			switch flag.ValueType() {
//NOFLAGYET 			case "int", "int8", "int16", "int32", "int64":
//NOFLAGYET 				val = cast.ToInt(flag.ValueString())
//NOFLAGYET 			case "bool":
//NOFLAGYET 				val = cast.ToBool(flag.ValueString())
//NOFLAGYET 			case "stringSlice":
//NOFLAGYET 				s := strings.TrimPrefix(flag.ValueString(), "[")
//NOFLAGYET 				s = strings.TrimSuffix(s, "]")
//NOFLAGYET 				val, _ = readAsCSV(s)
//NOFLAGYET 			case "intSlice":
//NOFLAGYET 				s := strings.TrimPrefix(flag.ValueString(), "[")
//NOFLAGYET 				s = strings.TrimSuffix(s, "]")
//NOFLAGYET 				res, _ := readAsCSV(s)
//NOFLAGYET 				val = cast.ToIntSlice(res)
//NOFLAGYET 			case "stringToString":
//NOFLAGYET 				val = stringToStringConv(flag.ValueString())
//NOFLAGYET 			default:
//NOFLAGYET 				val = flag.ValueString()
//NOFLAGYET 			}
//NOFLAGYET 			setKeyInMap(result, path, val)
//NOFLAGYET 		}
//NOFLAGYET 	}
//NOFLAGYET 	return
//NOFLAGYET }
