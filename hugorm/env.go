package hugorm

import (
	"fmt"
	"os"
	"strings"
)

// AutomaticEnv makes Hugorm check if environment variables match any of the existing keys
// (config, default or flags). If matching env vars are found, they are loaded into Hugorm.
func AutomaticEnv() { hg.AutomaticEnv() }

func (h *Hugorm) AutomaticEnv() {
	h.automaticEnvApplied = true
}

// SetEnvKeyReplacer sets the strings.Replacer.
// Useful for mapping an environmental variable to a key that does
// not match it.
func SetEnvKeyReplacer(r *strings.Replacer) { hg.SetEnvKeyReplacer(r) }

func (h *Hugorm) SetEnvKeyReplacer(r *strings.Replacer) {
	h.envKeyReplacer = r
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
	if h.envKeyReplacer != nil {
		key = h.envKeyReplacer.Replace(key)
	}

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
