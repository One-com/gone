package jconf

import (
	"bytes"
	"encoding/json"
	"testing"
)

type DurConfig struct {
	Delay Duration `json:"delay"`
}

var durdata = `{
    "delay": "17s"
}`

func TestDuration(t *testing.T) {

	var cfg *DurConfig

	buf := bytes.NewBufferString(durdata)

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

	if out.String() != durdata {
		t.Fail()
	}

}
