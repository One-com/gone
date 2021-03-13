package hugorm

import (
//"strings"
)

// Get can retrieve any value given the key to use.
// Get is case-insensitive for a key.
// Get has the behavior of returning the value associated with the first
// place from where it is set. Viper will check in the following order:
// override, flag, env, config file, key/value store, default
//
// Get returns an interface. For a specific value use one of the Get____ methods.
func Get(key string) interface{} { return hg.Get(key) }

func (h *Hugorm) Get(key string) interface{} {
	key = h.casing(key)

	val := h.find(key, true)
	if val == nil {
		return nil
	}

	//	if h.typeByDefValue {
	//		valType := val
	//		path := strings.Split(key, h.keyDelim)
	//		defVal := h.searchMap(h.defaults, path)
	//		if defVal != nil {
	//			valType = defVal
	//		}
	//
	//		switch valType.(type) {
	//		case bool:
	//			return cast.ToBool(val)
	//		case string:
	//			return cast.ToString(val)
	//		case int32, int16, int8, int:
	//			return cast.ToInt(val)
	//		case uint:
	//			return cast.ToUint(val)
	//		case uint32:
	//			return cast.ToUint32(val)
	//		case uint64:
	//			return cast.ToUint64(val)
	//		case int64:
	//			return cast.ToInt64(val)
	//		case float64, float32:
	//			return cast.ToFloat64(val)
	//		case time.Time:
	//			return cast.ToTime(val)
	//		case time.Duration:
	//			return cast.ToDuration(val)
	//		case []string:
	//			return cast.ToStringSlice(val)
	//		case []int:
	//			return cast.ToIntSlice(val)
	//		}
	//	}

	return val
}

//// GetString returns the value associated with the key as a string.
//func GetString(key string) string { return v.GetString(key) }
//
//func (v *Viper) GetString(key string) string {
//	return cast.ToString(v.Get(key))
//}
//
//// GetBool returns the value associated with the key as a boolean.
//func GetBool(key string) bool { return v.GetBool(key) }
//
//func (v *Viper) GetBool(key string) bool {
//	return cast.ToBool(v.Get(key))
//}
//
//// GetInt returns the value associated with the key as an integer.
//func GetInt(key string) int { return v.GetInt(key) }
//
//func (v *Viper) GetInt(key string) int {
//	return cast.ToInt(v.Get(key))
//}
//
//// GetInt32 returns the value associated with the key as an integer.
//func GetInt32(key string) int32 { return v.GetInt32(key) }
//
//func (v *Viper) GetInt32(key string) int32 {
//	return cast.ToInt32(v.Get(key))
//}
//
//// GetInt64 returns the value associated with the key as an integer.
//func GetInt64(key string) int64 { return v.GetInt64(key) }
//
//func (v *Viper) GetInt64(key string) int64 {
//	return cast.ToInt64(v.Get(key))
//}
//
//// GetUint returns the value associated with the key as an unsigned integer.
//func GetUint(key string) uint { return v.GetUint(key) }
//
//func (v *Viper) GetUint(key string) uint {
//	return cast.ToUint(v.Get(key))
//}
//
//// GetUint32 returns the value associated with the key as an unsigned integer.
//func GetUint32(key string) uint32 { return v.GetUint32(key) }
//
//func (v *Viper) GetUint32(key string) uint32 {
//	return cast.ToUint32(v.Get(key))
//}
//
//// GetUint64 returns the value associated with the key as an unsigned integer.
//func GetUint64(key string) uint64 { return v.GetUint64(key) }
//
//func (v *Viper) GetUint64(key string) uint64 {
//	return cast.ToUint64(v.Get(key))
//}
//
//// GetFloat64 returns the value associated with the key as a float64.
//func GetFloat64(key string) float64 { return v.GetFloat64(key) }
//
//func (v *Viper) GetFloat64(key string) float64 {
//	return cast.ToFloat64(v.Get(key))
//}
//
//// GetTime returns the value associated with the key as time.
//func GetTime(key string) time.Time { return v.GetTime(key) }
//
//func (v *Viper) GetTime(key string) time.Time {
//	return cast.ToTime(v.Get(key))
//}
//
//// GetDuration returns the value associated with the key as a duration.
//func GetDuration(key string) time.Duration { return v.GetDuration(key) }
//
//func (v *Viper) GetDuration(key string) time.Duration {
//	return cast.ToDuration(v.Get(key))
//}
//
//// GetIntSlice returns the value associated with the key as a slice of int values.
//func GetIntSlice(key string) []int { return v.GetIntSlice(key) }
//
//func (v *Viper) GetIntSlice(key string) []int {
//	return cast.ToIntSlice(v.Get(key))
//}
//
//// GetStringSlice returns the value associated with the key as a slice of strings.
//func GetStringSlice(key string) []string { return v.GetStringSlice(key) }
//
//func (v *Viper) GetStringSlice(key string) []string {
//	return cast.ToStringSlice(v.Get(key))
//}
//
//// GetStringMap returns the value associated with the key as a map of interfaces.
//func GetStringMap(key string) map[string]interface{} { return v.GetStringMap(key) }
//
//func (v *Viper) GetStringMap(key string) map[string]interface{} {
//	return cast.ToStringMap(v.Get(key))
//}
//
//// GetStringMapString returns the value associated with the key as a map of strings.
//func GetStringMapString(key string) map[string]string { return v.GetStringMapString(key) }
//
//func (v *Viper) GetStringMapString(key string) map[string]string {
//	return cast.ToStringMapString(v.Get(key))
//}
//
//// GetStringMapStringSlice returns the value associated with the key as a map to a slice of strings.
//func GetStringMapStringSlice(key string) map[string][]string { return v.GetStringMapStringSlice(key) }
//
//func (v *Viper) GetStringMapStringSlice(key string) map[string][]string {
//	return cast.ToStringMapStringSlice(v.Get(key))
//}
//
//// GetSizeInBytes returns the size of the value associated with the given key
//// in bytes.
//func GetSizeInBytes(key string) uint { return v.GetSizeInBytes(key) }
//
//func (v *Viper) GetSizeInBytes(key string) uint {
//	sizeStr := cast.ToString(v.Get(key))
//	return parseSizeInBytes(sizeStr)
//}
//
