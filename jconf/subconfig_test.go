package jconf

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"testing"
)

type FullConfig struct {
	M *AppConfig
	O *AppConfigOpt
	X *OptionalSubConfig
}

type simpleConfig map[string]string

type AppConfig struct {
	A string
	S *MandatorySubConfig
}

type AppConfigOpt struct {
	A string
	S *OptionalSubConfig `json:",omitempty"`
}

type ModuleConfig struct {
	B string
}

var confdata = `{ "a" : "app", "s" : {"b": "module"}}`

func initSubModule(cfg SubConfig) (err error) {
	var jc *ModuleConfig
	err = cfg.ParseInto(&jc)
	return
}

func initSimpleSubModule(cfg SubConfig) (err error) {
	var jc interface{}
	err = cfg.ParseInto(&jc)
	return
}

func initSubModuleNoPointer(cfg SubConfig) (err error) {
	var jc ModuleConfig
	err = cfg.ParseInto(&jc)
	return
}

func ExampleSubConfig() {

	// main application conf object
	cfg := &AppConfig{}

	// make a JSON decoder
	dec := json.NewDecoder(bytes.NewBuffer([]byte(confdata)))
	dec.UseNumber()

	// Parse the man config
	err := dec.Decode(&cfg)
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	// Let our submodule parse its own config
	err = initSubModule(cfg.S)
	if err != nil {
		log.Fatalf("Module Config parsing failed: %s", err.Error())
	}

	var out bytes.Buffer
	b, err := json.Marshal(cfg)
	if err != nil {
		log.Fatalf("Marshal error: %s", err)
	}

	err = json.Indent(&out, b, "", "    ")
	if err != nil {
		log.Fatalf("Indent error: %s", err)
	}
	out.WriteTo(os.Stdout)

	// Output:
	// {
	//     "A": "app",
	//     "S": {
	//         "B": "module"
	//     }
	// }
}

func TestSubConfigNoPointer(t *testing.T) {

	// main application conf object
	cfg := &AppConfig{}

	// make a JSON decoder
	dec := json.NewDecoder(bytes.NewBuffer([]byte(confdata)))
	dec.UseNumber()

	// Parse the man config
	err := dec.Decode(&cfg)
	if err != nil {
		t.Fatalf("%v\n", err)
	}

	// Let our submodule parse its own config
	err = initSubModuleNoPointer(cfg.S)
	if err != nil {
		t.Fatalf("Module Config parsing failed: %s", err.Error())
	}

	var out bytes.Buffer
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %s", err)
	}

	err = json.Indent(&out, b, "", "    ")
	if err != nil {
		t.Fatalf("Indent error: %s", err)
	}

	expected := `{
    "A": "app",
    "S": {
        "B": "module"
    }
}`
	output := out.String()
	if output != expected {
		t.Errorf("Unexpected output:\n%s\nexpected:\n%s\n", output, expected)
	}

}

//------------------------------------------------------------------

var fullconfdata = `{
    "M": {
        "A": "app",
        "S": {
            "B": "xy"
        }
    },
    "O": {
        "A": "foo",
        "S": {
            "B": "bar"
        }
    },
    "X": { // a string/string map
        "key": "val"
    }
}
`

func TestFull(t *testing.T) {

	// main application conf object
	var cfg *FullConfig

	buf := bytes.NewBufferString(fullconfdata)

	err := ParseInto(buf, &cfg)
	if err != nil {
		t.Fatalf("%v\n", err)
	}

	// Let our submodule parse its own config
	msub := cfg.M
	err = initSubModule(msub.S)
	if err != nil {
		t.Fatalf("Module Config parsing failed: %s", err.Error())
	}

	// also for the non existing optional one
	osub := cfg.O
	if osub != nil {
		err = initSubModule(osub.S)
		if err != nil {
			t.Fatalf("Module Config parsing failed: %s", err.Error())
		}
	}

	// and the kv map
	err = initSimpleSubModule(cfg.X)
	if err != nil {
		t.Fatalf("Module Config parsing failed: %s", err.Error())
	}

	var out bytes.Buffer
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %s", err.Error())
	}

	err = json.Indent(&out, b, "", "    ")
	if err != nil {
		log.Fatalf("Indent error: %s", err.Error())
	}

	expected := `{
    "M": {
        "A": "app",
        "S": {
            "B": "xy"
        }
    },
    "O": {
        "A": "foo",
        "S": {
            "B": "bar"
        }
    },
    "X": {
        "key": "val"
    }
}`

	output := out.String()
	if output != expected {
		t.Errorf("Unexpected output:\n%s\nexpected:\n%s\n", output, expected)
	}
}

var confdata3 = `// start comment
{
"a" : "app",
// comment
"s" : {
   "b", : "x // y" // end line comment
  },
}`

func TestSyntaxError(t *testing.T) {

	// main application conf object
	cfg := &AppConfig{}

	buf := bytes.NewBufferString(confdata3)

	err := ParseInto(buf, &cfg)
	if err == nil {

	}

	var out = bytes.NewBufferString(err.Error())

	expected := `Parse error: invalid character ',' after object key (byte=58 line=6):    "b",<---`

	output := out.String()
	if output != expected {
		t.Errorf("Unexpected output:\n%s\nexpected:\n%s\n", output, expected)
	}
}

var nilconfdata = `{ "a" : "app" }`

func TestMandatoryNilSubConfig(t *testing.T) {

	// main application conf object
	cfg := &AppConfig{}

	// make a JSON decoder
	dec := json.NewDecoder(bytes.NewBuffer([]byte(nilconfdata)))
	dec.UseNumber()

	// Parse the man config
	err := dec.Decode(&cfg)
	if err != nil {
		t.Fatalf("%v\n", err)
	}

	// Let our submodule parse its own config
	err = initSubModule(cfg.S)
	if err == nil {
		t.Fatalf("Module Config parsing didn't detect error")
	} else {
		if err != ErrEmptySubConfig {
			t.Fatalf("Module Config parsing failed: %s", err.Error())
		}
	}

	var out bytes.Buffer
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %s", err)
	}

	err = json.Indent(&out, b, "", "    ")
	if err != nil {
		t.Fatalf("Indent error: %s", err)
	}

	expected := `{
    "A": "app",
    "S": null
}`

	output := out.String()
	if output != expected {
		t.Errorf("Unexpected output:\n%s\nexpected:\n%s\n", output, expected)
	}

}

func TestOptionalNilSubConfig(t *testing.T) {

	// main application conf object
	cfg := &AppConfigOpt{}

	// make a JSON decoder
	dec := json.NewDecoder(bytes.NewBuffer([]byte(nilconfdata)))
	dec.UseNumber()

	// Parse the man config
	err := dec.Decode(&cfg)
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	// Let our submodule parse its own config
	err = initSubModule(cfg.S)
	if err != nil {
		log.Fatalf("Module Config parsing failed: %s", err.Error())
	}

	var out bytes.Buffer
	b, err := json.Marshal(cfg)
	if err != nil {
		log.Fatalf("Marshal error: %s", err)
	}

	err = json.Indent(&out, b, "", "    ")
	if err != nil {
		log.Fatalf("Indent error: %s", err)
	}

	expected := `{
    "A": "app"
}`

	output := out.String()
	if output != expected {
		t.Errorf("Unexpected output:\n%s\nexpected:\n%s\n", output, expected)
	}
}
