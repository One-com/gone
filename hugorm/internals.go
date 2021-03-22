package hugorm

import (
	"strings"
)

// mergeFlatMap merges the given maps, excluding values of the second map
// shadowed by values from the first map.
func (h *Hugorm) mergeFlatMap(shadow map[string]bool, m map[string]interface{}) map[string]bool {
	// scan keys
outer:
	for k := range m {
		path := strings.Split(k, h.keyDelim)
		// scan intermediate paths
		var parentKey string
		for i := 1; i < len(path); i++ {
			parentKey = strings.Join(path[0:i], h.keyDelim)
			if shadow[parentKey] {
				// path is shadowed, continue
				continue outer
			}
		}
		// add key
		shadow[strings.ToLower(k)] = true
	}
	return shadow
}

func (h *Hugorm) find(key string, flagDefault bool) interface{} {
	var (
		val interface{}
		//		exists bool
		path = strings.Split(key, h.keyDelim)
		//		nested = len(path) > 1
	)

	//	// compute the path through the nested maps to the nested value
	//	if nested && v.isPathShadowedInDeepMap(path, castMapStringToMapInterface(v.aliases)) != "" {
	//		return nil
	//	}

	// if the requested key is an alias, then return the proper key
	key = h.realKey(key)
	path = strings.Split(key, h.keyDelim)
	//	nested = len(path) > 1

	var config map[string]interface{}
	config = h.Config()

	//fmt.Println("CONFIG", config)
	val = searchMap(config, path)
	if val != nil {
		//	fmt.Println("VAL NIL", key)
		return val
	}

	//	if nested && v.isPathShadowedInDeepMap(path, v.override) != "" {
	//		return nil
	//	}

	//	// PFlag override next
	//	flag, exists := v.pflags[lcaseKey]
	//	if exists && flag.HasChanged() {
	//		switch flag.ValueType() {
	//		case "int", "int8", "int16", "int32", "int64":
	//			return cast.ToInt(flag.ValueString())
	//		case "bool":
	//			return cast.ToBool(flag.ValueString())
	//		case "stringSlice":
	//			s := strings.TrimPrefix(flag.ValueString(), "[")
	//			s = strings.TrimSuffix(s, "]")
	//			res, _ := readAsCSV(s)
	//			return res
	//		case "intSlice":
	//			s := strings.TrimPrefix(flag.ValueString(), "[")
	//			s = strings.TrimSuffix(s, "]")
	//			res, _ := readAsCSV(s)
	//			return cast.ToIntSlice(res)
	//		case "stringToString":
	//			return stringToStringConv(flag.ValueString())
	//		default:
	//			return flag.ValueString()
	//		}
	//	}
	//	if nested && v.isPathShadowedInFlatMap(path, v.pflags) != "" {
	//		return nil
	//	}
	//
	//	// Env override next
	//	if v.automaticEnvApplied {
	//		// even if it hasn't been registered, if automaticEnv is used,
	//		// check any Get request
	//		if val, ok := v.getEnv(v.mergeWithEnvPrefix(lcaseKey)); ok {
	//			return val
	//		}
	//		if nested && v.isPathShadowedInAutoEnv(path) != "" {
	//			return nil
	//		}
	//	}
	//	envkeys, exists := v.env[lcaseKey]
	//	if exists {
	//		for _, envkey := range envkeys {
	//			if val, ok := v.getEnv(envkey); ok {
	//				return val
	//			}
	//		}
	//	}
	//	if nested && v.isPathShadowedInFlatMap(path, v.env) != "" {
	//		return nil
	//	}
	//
	//	// Config file next
	//	val = v.searchIndexableWithPathPrefixes(v.config, path)
	//	if val != nil {
	//		return val
	//	}
	//	if nested && v.isPathShadowedInDeepMap(path, v.config) != "" {
	//		return nil
	//	}
	//
	//	// K/V store next
	//	val = v.searchMap(v.kvstore, path)
	//	if val != nil {
	//		return val
	//	}
	//	if nested && v.isPathShadowedInDeepMap(path, v.kvstore) != "" {
	//		return nil
	//	}
	//
	//	// Default next
	//	val = v.searchMap(v.defaults, path)
	//	if val != nil {
	//		return val
	//	}
	//	if nested && v.isPathShadowedInDeepMap(path, v.defaults) != "" {
	//		return nil
	//	}
	//
	//	if flagDefault {
	//		// last chance: if no value is found and a flag does exist for the key,
	//		// get the flag's default value even if the flag's value has not been set.
	//		if flag, exists := v.pflags[lcaseKey]; exists {
	//			switch flag.ValueType() {
	//			case "int", "int8", "int16", "int32", "int64":
	//				return cast.ToInt(flag.ValueString())
	//			case "bool":
	//				return cast.ToBool(flag.ValueString())
	//			case "stringSlice":
	//				s := strings.TrimPrefix(flag.ValueString(), "[")
	//				s = strings.TrimSuffix(s, "]")
	//				res, _ := readAsCSV(s)
	//				return res
	//			case "intSlice":
	//				s := strings.TrimPrefix(flag.ValueString(), "[")
	//				s = strings.TrimSuffix(s, "]")
	//				res, _ := readAsCSV(s)
	//				return cast.ToIntSlice(res)
	//			case "stringToString":
	//				return stringToStringConv(flag.ValueString())
	//			default:
	//				return flag.ValueString()
	//			}
	//		}
	//		// last item, no need to check shadowing
	//	}
	//
	return nil
}
