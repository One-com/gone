package jconf

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
)

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

func ExampleSubConfigNoPointer() {

	// main application conf object
	cfg := &AppConfig{}

	// make a JSON decoder
	dec := json.NewDecoder(bytes.NewBuffer([]byte(confdata)))
	dec.UseNumber()

	// Parse the man config
	err := dec.Decode(&cfg)
	if err != nil {
		log.Fatalf("%#v\n", err)
	}

	// Let our submodule parse its own config
	err = initSubModuleNoPointer(cfg.S)
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

//------------------------------------------------------------------

func ExampleFull() {

	// main application conf object
	cfg := &AppConfig{}

	buf := bytes.NewBufferString(confdata2)

	err := ParseInto(buf, &cfg)
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
		log.Fatalf("Marshal error: %s", err.Error())
	}

	err = json.Indent(&out, b, "", "    ")
	if err != nil {
		log.Fatalf("Indent error: %s", err.Error())
	}
	out.WriteTo(os.Stdout)

	// Output:
	// {
	//     "A": "app",
	//     "S": {
	//         "B": "x // y"
	//     }
	// }
}

var confdata3 = `// start comment
{
"a" : "app",
// comment
"s" : {
   "b", : "x // y" // end line comment
  },
}`

func ExampleSyntaxError() {

	// main application conf object
	cfg := &AppConfig{}

	buf := bytes.NewBufferString(confdata3)

	err := ParseInto(buf, &cfg)
	if err == nil {

	}

	var out = bytes.NewBufferString(err.Error())

	out.WriteTo(os.Stdout)

	// Output:
	// Parse error: invalid character ',' after object key (byte=58 line=6):    "b",<---
}

var nilconfdata = `{ "a" : "app" }`

func ExampleMandatoryNilSubConfig() {

	// main application conf object
	cfg := &AppConfig{}

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
	if err == nil {
		log.Fatalf("Module Config parsing didn't detect error")
	} else {
		if err != ErrEmptySubConfig {
			log.Fatalf("Module Config parsing failed: %s", err.Error())
		}
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
	//     "S": null
	// }
}

func ExampleOptionalNilSubConfig() {

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
	out.WriteTo(os.Stdout)

	// Output:
	// {
	//     "A": "app"
	// }
}
