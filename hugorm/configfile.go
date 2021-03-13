package hugorm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v2"
	"io"
	"os"
	"strings"
)

// ConfigParseError denotes failing to parse configuration file.
type ConfigParseError struct {
	err error
}

// Error returns the formatted configuration error.
func (pe ConfigParseError) Error() string {
	return fmt.Sprintf("While parsing config: %s", pe.err.Error())
}

// make a type implementing the io.fs.FS interface based on pkg/os
//----
type OSFS struct{}

func (osfs *OSFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

//-----
type inMem struct {
	values map[string]interface{}
}

func (c *inMem) Values() map[string]interface{} {
	return deepCopyMap(c.values, false)
}

//-----
type File struct {
	filetype string
	filename string
	values   map[string]interface{}
}

func (c *File) Values() map[string]interface{} {
	return deepCopyMap(c.values, false)
}

func (c *File) Load() (err error) {
	var data []byte

	fs := &OSFS{}

	data, err = fs.ReadFile(c.filename)
	if err != nil {
		return
	}

	config := make(map[string]interface{})

	err = unmarshalReader(c.filetype, bytes.NewReader(data), config)
	if err != nil {
		return err
	}

	c.values = config

	return nil
}

func unmarshalReader(format string, in io.Reader, c map[string]interface{}) error {
	buf := new(bytes.Buffer)
	buf.ReadFrom(in)

	switch strings.ToLower(format) {
	case "yaml", "yml":
		if err := yaml.Unmarshal(buf.Bytes(), &c); err != nil {
			return ConfigParseError{err}
		}

	case "json":
		if err := json.Unmarshal(buf.Bytes(), &c); err != nil {
			return ConfigParseError{err}
		}

	case "toml":
		tree, err := toml.LoadReader(buf)
		if err != nil {
			return ConfigParseError{err}
		}
		tmap := tree.ToMap()
		for k, v := range tmap {
			c[k] = v
		}
	default:
		return ConfigParseError{
			err: fmt.Errorf("Unknown format: %s", format),
		}
	}

	// TODO
	//insensitiviseMap(c)
	return nil
}

// Path stuff?
//func (v *Viper) searchInPath(in string) (filename string) {
//
//	for _, ext := range SupportedExts {
//		jww.DEBUG.Println("Checking for", filepath.Join(in, v.configName+"."+ext))
//		if b, _ := exists(v.fs, filepath.Join(in, v.configName+"."+ext)); b {
//			jww.DEBUG.Println("Found: ", filepath.Join(in, v.configName+"."+ext))
//			return filepath.Join(in, v.configName+"."+ext)
//		}
//	}
//
//	if v.configType != "" {
//		if b, _ := exists(v.fs, filepath.Join(in, v.configName)); b {
//			return filepath.Join(in, v.configName)
//		}
//	}
//
//	return ""
//}
//
//// Search all configPaths for any config file.
//// Returns the first path that exists (and is a config file).
//func (v *Viper) findConfigFile() (string, error) {
//
//
//	for _, cp := range v.configPaths {
//		file := v.searchInPath(cp)
//		if file != "" {
//			return file, nil
//		}
//	}
//	return "", ConfigFileNotFoundError{v.configName, fmt.Sprintf("%s", v.configPaths)}
//}
