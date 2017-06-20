package jconf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

type DurConfig struct {
	Delay     Duration `json:"delay"`
	NanoDelay Duration `json:"nanodelay"`
}

var indata = `{
    "delay": "17s",
    "nanodelay": 31000
}`

var outdata = `{
    "delay": "17s",
    "nanodelay": "31µs"
}`

func TestDuration(t *testing.T) {

	var cfg *DurConfig

	buf := bytes.NewBufferString(indata)

	err := ParseInto(buf, &cfg)
	if err != nil {
		t.Fatalf("%v\n", err)
	}

	var out bytes.Buffer
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %s", err.Error())
	}

	err = json.Indent(&out, b, "", "    ")
	if err != nil {
		t.Fatalf("Indent error: %s", err.Error())
	}

	result := out.String()
	if result != outdata {
		fmt.Println(result)
		t.Fail()
	}

}
