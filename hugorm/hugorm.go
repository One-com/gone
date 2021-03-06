// The below code is heavily derived from github.com/spf13/viper,
// which comes with the below copyright notice:
//
// Copyright © 2014 Steve Francia <spf@spf13.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Hugorm is an application configuration system.
// It believes that applications can be configured a variety of ways
// via flags, ENVIRONMENT variables, configuration files retrieved
// from the file system, or a remote key/value store.

// Each item takes precedence over the item below it:

// overrides
// flag (TODO)
// env
// config
// key/value store
// default

// Hugorm is a replacement for the gone/jconf library which was just a thin
// wrapper around stdlib encoding/json. That direct coupling to the stdlib json
// parser turned out to be a limitation for features and Hugorm (like Viper) employs
// an intermediate representation of the config as a map[string]interface{} instead.

package hugorm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"strings"
	// TODO make go-routine safe
	// "sync/atomic"
)

// ConfigMarshalError happens when failing to marshal the configuration.
type ConfigMarshalError struct {
	err error
}

// Error returns the formatted configuration error.
func (e ConfigMarshalError) Error() string {
	return fmt.Sprintf("While marshaling config: %s", e.err.Error())
}

// Global instance
var hg *Hugorm

// GetGlobal gets the global Hugorm instance.
func GetGlobal() *Hugorm {
	return hg
}

func init() {
	Reset()
}

// Reset is intended for testing, will reset all to default settings.
// In the public interface for the viper package so applications
// can use it in their testing as well.
func Reset(opts ...Option) {
	hg = New(opts...)
	//APM   SupportedExts = []string{"json", "yaml", "yml"}
	//APM   SupportedRemoteProviders = []string{"etcd", "consul"}
}

// ConfigSource is a case sensitive recursive store of config key/value (values can be maps)
type ConfigSource interface {
	Values() map[string]interface{}
}

// ConfigLoader is a ConfigSource which needs explicit loading to refresh it's values
type ConfigLoader interface {
	ConfigSource
	Load() error
}

// Hugorm is a prioritized configuration registry. It
// maintains a set of configuration sources, fetches
// values to populate those, and provides them according
// to the source's priority.
// The priority of the sources is the following:
// 1. overrides (see the Set() function)
// 2. flags
// 3. env. variables
// 4. other config sources, - per default a file in a supported format
// 5. defaults (see the SetDefault() function)
//
// Config sources can be hierarchical (like a JSON file), but each value
// still has a unique key in a flat keyspace. (using a key-delimiter to define it's path)
//
// So - given a key-delimiter of "." the following will be true:
//
//  JSON config:
//  {
//    "foo" : {
//       "bar": "baz"
//    }
//  }
//
// key "foo.bar" == "baz"
//
type Hugorm struct {
	// Delimiter that separates a list of keys
	// used to access a nested value in one go
	keyDelim  string
	envPrefix string

	// config keys stored here are case sensitive.
	//flags map[string]FlagValue
	env map[string][]string

	override map[string]interface{}
	defaults map[string]interface{}

	// prioritized list of config sources
	sources []ConfigSource

	//onConfigChange func(fsnotify.Event)

	// Cache of the consolidated config. Needs invalidation every time
	// some source makes values change.
	configCache map[string]interface{}

	allowEmptyEnv bool

	caseInsensitive bool
}

// A few util functions
func (h *Hugorm) casing(key string) string {
	if h.caseInsensitive {
		return strings.ToLower(key)
	}
	return key
}

// Right now we don't support aliases.
func (h *Hugorm) realKey(key string) string {
	//newkey, exists := v.aliases[key]
	//if exists {
	//        return v.realKey(newkey)
	//}
	return key
}

//------------- cache access --------------------
func (h *Hugorm) invalidateCache() {
	// TODO: go-routine safe
	h.configCache = nil
}

//---------------------------------- OPTIONS ------------------------------

type Option interface {
	apply(h *Hugorm)
}

type optionFunc func(h *Hugorm)

func (fn optionFunc) apply(h *Hugorm) {
	fn(h)
}

// KeyDelimiter sets the delimiter used for determining key parts.
// By default it's value is ".".
func KeyDelimiter(d string) Option {
	return optionFunc(func(h *Hugorm) {
		h.keyDelim = d
	})
}

func EnvPrefix(pfx string) Option {
	return optionFunc(func(h *Hugorm) {
		h.envPrefix = pfx
	})
}

func CaseSensitive(sensitive bool) Option {
	return optionFunc(func(h *Hugorm) {
		h.caseInsensitive = !sensitive
	})
}

func ConfigFile(format, name string) Option {
	return optionFunc(func(h *Hugorm) {
		h.sources = append(h.sources, &File{filename: name, filetype: format})
	})
}

// New returns an initialized Viper instance.
func New(opts ...Option) *Hugorm {
	h := new(Hugorm)

	h.keyDelim = "."

	h.override = make(map[string]interface{})
	h.defaults = make(map[string]interface{})

	//h.flags = make(map[string]FlagValue)
	h.env = make(map[string][]string)

	for _, opt := range opts {
		opt.apply(h)
	}

	return h
}

//------------------------------------------------------------------------------

func AddConfigFile(format, filename string) { hg.AddConfigFile(format, filename) }

func (h *Hugorm) AddConfigFile(format, filename string) {
	if filename != "" {
		h.sources = append(h.sources,
			&File{
				filename: filename,
				filetype: format,
			})
	}
	h.invalidateCache()
}

//// TODO
//// IsSet checks to see if the key has been set in any of the data locations.
//// IsSet is case-insensitive for a key.
//func IsSet(key string) bool { return hg.IsSet(key) }
//
//func (h *Hugorm) IsSet(key string) bool {
//      key = h.casing(key)
//	val := v.find(key, false)
//	return val != nil
//}

// support deep keys// // InConfig checks to see if the given key (or an alias) is in the config file.
// support deep keys// func InConfig(key string) bool { return hg.InConfig(key) }
// support deep keys//
// support deep keys// func (h *Hugorm) InConfig(key string) bool {
// support deep keys// 	key = h.casing(key)
// support deep keys//
// support deep keys// 	// if the requested key is an alias, then return the proper key
// support deep keys// 	key = h.realKey(key)
// support deep keys//
// support deep keys// 	config := h.Config()
// support deep keys//
// support deep keys// 	_, exists := config[key]
// support deep keys// 	return exists
// support deep keys// }

// SetDefault sets the default value for this key.
// Default only used when no value is provided by the user via flag, config or ENV.
func SetDefault(key string, value interface{}) { hg.SetDefault(key, value) }

func (h *Hugorm) SetDefault(key string, value interface{}) {
	// If alias passed in, then set the proper default
	key = h.realKey(h.casing(key))
	//value = toCaseInsensitiveValue(value)

	path := strings.Split(key, h.keyDelim)
	setKeyInMap(h.defaults, path, value)

	h.invalidateCache()
}

// Set sets the value for the key in the override register.
// Set is case-insensitive for a key.
// Will be used instead of values obtained via
// flags, config file, ENV, default, or key/value store.
func Set(key string, value interface{}) { hg.Set(key, value) }

func (h *Hugorm) Set(key string, value interface{}) {
	// If alias passed in, then set the proper override
	key = h.realKey(h.casing(key))
	//value = toCaseInsensitiveValue(value)

	path := strings.Split(key, h.keyDelim)
	setKeyInMap(h.override, path, value)

	h.invalidateCache()
}

// LoadConfig will discover and load the configuration file from disk
// and key/value stores, searching in one of the defined paths.
func LoadConfig() error { return hg.LoadConfig() }

func (h *Hugorm) LoadConfig() error {

	for _, s := range h.sources {
		if l, ok := s.(ConfigLoader); ok {
			err := l.Load()
			if err != nil {
				return err
			}
		}

	}

	h.invalidateCache()

	return nil
}

// AddConfigFrom will parse the data in the provided io.Reader and append it to the config.
func AddConfigFrom(format string, in io.Reader) error { return hg.AddConfigFrom(format, in) }

func (h *Hugorm) AddConfigFrom(format string, in io.Reader) error {

	var data = make(map[string]interface{})

	err := unmarshalReader(format, in, data)
	if err != nil {
		return err
	}

	return h.AddConfigMap(data)
}

// AddConfigMap appends the configuration from the map given as a constant config source
func AddConfigMap(cfg map[string]interface{}) error { return hg.AddConfigMap(cfg) }

func (h *Hugorm) AddConfigMap(cfg map[string]interface{}) error {

	h.sources = append(h.sources, &inMem{values: cfg})

	h.invalidateCache()

	return nil
}

// WriteConfig writes the current configuration to a file.
func WriteConfigTo(out io.Writer) error { return hg.WriteConfigTo(out) }

func (h *Hugorm) WriteConfigTo(out io.Writer) error {
	err := h.marshalWriter(out, "json")
	return err
}

// Marshal (alias for WriteConfigTo)
func Marshal(out io.Writer) error { return hg.Marshal(out) }

func (h *Hugorm) Marshal(out io.Writer) error {
	return h.WriteConfigTo(out)
}

// Marshal a map into Writer.
func (h *Hugorm) marshalWriter(out io.Writer, configType string) error {
	f := bufio.NewWriter(out)
	c := h.Config()
	switch configType {
	case "json":
		b, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			return ConfigMarshalError{err}
		}
		_, err = f.WriteString(string(b))
		if err != nil {
			return ConfigMarshalError{err}
		}
		//	case "toml":
		//		t, err := toml.TreeFromMap(c)
		//		if err != nil {
		//			return ConfigMarshalError{err}
		//		}
		//		s := t.String()
		//		if _, err := f.WriteString(s); err != nil {
		//			return ConfigMarshalError{err}
		//		}
		//
	case "yaml", "yml":
		b, err := yaml.Marshal(c)
		if err != nil {
			return ConfigMarshalError{err}
		}
		if _, err = f.WriteString(string(b)); err != nil {
			return ConfigMarshalError{err}
		}
	default:
		return ConfigMarshalError{fmt.Errorf("Unknown configType: '%s'", configType)}
	}
	f.Flush()
	return nil
}

//// AllKeys returns all keys holding a value, regardless of where they are set.
//// Nested keys are returned with a v.keyDelim separator
//func AllKeys() []string { return hg.AllKeys() }
//
//func (h *Hugorm) AllKeys() []string {
//	m := map[string]bool{}
//      TODO: traverse config getting keys....
//	// convert set of paths to list
//	a := make([]string, 0, len(m))
//	for x := range m {
//		a = append(a, x)
//	}
//	return a
//}

// Config returns the consolidated config
func Config() map[string]interface{} { return hg.Config() }

func (h *Hugorm) Config() map[string]interface{} {
	if h.configCache == nil {
		h.configCache = h.consolidateConfigs()
	}
	return h.configCache
}

// consolidateConfigs takes all the config sources in priority (flags, env, sources..)
// and computes the union of keys, overwriting earlier keys with later.
func (h *Hugorm) consolidateConfigs() (consolidated map[string]interface{}) {

	// merge in priority order - lowest first.
	consolidated = deepCopyMap(h.defaults, h.caseInsensitive)

	for _, s := range h.sources {
		mcopy := deepCopyMap(s.Values(), h.caseInsensitive)
		mergeMaps(consolidated, mcopy)
	}

	// Environment
	mergeMaps(consolidated, h.envBindings2configMap(h.env))

	// Flags
	//mergeMaps(consolidated, h.flagBindings2configMap(h.flags))

	// Override
	mcopy := deepCopyMap(h.override, h.caseInsensitive)
	mergeMaps(consolidated, mcopy)

	return
}

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
	return val
}

// Debug prints all configuration registries for debugging
// purposes.
func Debug() { hg.Debug() }

func (h *Hugorm) Debug() {
	fmt.Printf("Defaults:\n%#v\n", h.defaults)
	for s := range h.sources {
		fmt.Printf("Source:\n%#v\n", s)
	}
	fmt.Printf("Override:\n%#v\n", h.override)

	//fmt.Printf("Flags:\n%#v\n", h.flags)
	fmt.Printf("Env:\n%#v\n", h.env)
}
