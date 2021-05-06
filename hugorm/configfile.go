package hugorm

import (
	"bytes"
	"encoding/json"
	"fmt"
	//"github.com/pelletier/go-toml"
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

	data, err = os.ReadFile(c.filename)
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

		//	case "toml":
		//		tree, err := toml.LoadReader(buf)
		//		if err != nil {
		//			return ConfigParseError{err}
		//		}
		//		tmap := tree.ToMap()
		//		for k, v := range tmap {
		//			c[k] = v
		//		}

	default:
		return ConfigParseError{
			err: fmt.Errorf("Unknown format: %s", format),
		}
	}

	return nil
}
