package jconf

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
)

type AppConfig struct {
	A string
	S *SubConfig
}

type ModuleConfig struct {
	B string
}

var confdata = `{ "a" : "app", "s" : {"b": "module"}}`

func initSubModule(cfg *SubConfig) {
	var jc *ModuleConfig
	err := cfg.ParseInto(&jc)
	if err != nil {
		log.Fatal("Module Config parsing failed")
	}
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
		log.Fatalf("%#v\n", err)
	}

	// Let our submodule parse its own config
	initSubModule(cfg.S)

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
