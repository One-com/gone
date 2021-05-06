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

// find a key (in it's path form) in the config. Fall back to
// flag default values if flagDefault==true
// find is always case sensitive.
func (h *Hugorm) find(key string, flagDefault bool) interface{} {

	var val interface{}

	// if the requested key is an alias, then return the proper key
	key = h.realKey(key)

	path := strings.Split(key, h.keyDelim)

	config := h.Config()

	val = searchMap(config, path)
	return val
}
