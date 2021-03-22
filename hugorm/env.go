package hugorm

import (
	"fmt"
	"os"
	"strings"
)

// SetEnvPrefix defines a prefix that ENVIRONMENT variables will use.
// E.g. if your prefix is "spf", the env registry will look for env
// variables that start with "SPF_".
func SetEnvPrefix(in string) { hg.SetEnvPrefix(in) }

func (h *Hugorm) SetEnvPrefix(in string) {
	if in != "" {
		h.envPrefix = in
	}
}

// BindEnv binds a Hugorm key to a ENV variable.
// ENV variables are case sensitive.
// If only a key is provided, it will use the env key matching the key, uppercased.
// If more arguments are provided, they will represent the env variable names that
// should bind to this key and will be taken in the specified order.
// EnvPrefix will be used when set when env name is not provided.
func BindEnv(input ...string) error { return hg.BindEnv(input...) }

func (h *Hugorm) BindEnv(input ...string) error {
	if len(input) == 0 {
		return fmt.Errorf("missing key to bind to")
	}

	key := h.casing(input[0])

	if len(input) == 1 {
		// Only key provided, convert it to an ENV var name
		h.env[key] = append(h.env[key], h.withEnvPrefix(key))
	} else {
		// Take the provided names verbatim.
		h.env[key] = append(h.env[key], input[1:]...)
	}

	h.invalidateCache()

	return nil
}

func (h *Hugorm) withEnvPrefix(in string) string {
	if h.envPrefix != "" {
		return strings.ToUpper(h.envPrefix + "_" + in)
	}

	return strings.ToUpper(in)
}

// getEnv is a wrapper around os.Getenv which replaces characters in the original
// key. This allows env vars which have different keys than the config object
// keys.
func (h *Hugorm) getEnv(key string) (string, bool) {
	val, ok := os.LookupEnv(key)

	return val, ok && (h.allowEmptyEnv || val != "")
}

// AllowEmptyEnv tells Hugorm to consider set,
// but empty environment variables as valid values instead of falling back.
// For backward compatibility reasons this is false by default.
func AllowEmptyEnv(allowEmptyEnv bool) { hg.AllowEmptyEnv(allowEmptyEnv) }

func (h *Hugorm) AllowEmptyEnv(allowEmptyEnv bool) {
	h.allowEmptyEnv = allowEmptyEnv
}

func (h *Hugorm) envBindings2configMap(bindings map[string][]string) (result map[string]interface{}) {

	result = make(map[string]interface{})

BINDING:
	for b, envkeys := range bindings {
		path := strings.Split(b, h.keyDelim)
		for _, envkey := range envkeys {
			if val, ok := h.getEnv(envkey); ok {
				setKeyInMap(result, path, val)
				continue BINDING
			}
		}

	}
	return
}
