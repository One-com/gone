package jconf

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
)

type MyConfig struct {
	A string
	S FurtherConfig
}

type FurtherConfig struct {
	B string
}

var confdata2 = `// start comment
{
"a" : "app",
// comment
"s" : {
   "b" : "x // y" // end line comment
  }
}`

func ExampleParseInto() {

	// main application conf object
	cfg := &MyConfig{}

	buf := bytes.NewBufferString(confdata2)

	err := ParseInto(buf, &cfg)
	if err != nil {
		log.Fatalf("%#v\n", err)
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
