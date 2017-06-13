# gone/jconf

[![GoDoc](https://godoc.org/github.com/one-com/gone/jconf?status.svg)](https://godoc.org/github.com/one-com/gone/jconf) [![GoReportCard](https://goreportcard.com/badge/github.com/One-com/gone)](https://goreportcard.com/report/github.com/One-com/gone/jconf) [Coverage](http://gocover.io/github.com/One-com/gone/jconf)

Modular JSON config parsing allowing // comments and full serialization of the entire resulting data hierarchy to JSON.

Package jconf allows you to avoid having you main configuration data structure know about every individual code modules internal
configuration structures in order to serialize the entire configuration. Instead you can localize module config data structures
with the code using them.

## Example

Below is a complete example using the comment-filtering pre-processor:

```go
import (
	"github.com/One-com/gone/jconf"
	"encoding/json"
	"os"
	"bytes"
	"log"
)

type AppConfig struct {
	A string
	S *jconf.MandatorySubConfig
}

type ModuleConfig struct {
	B string
}

func initSubModule(cfg jconf.SubConfig) {
	var jc *ModuleConfig
	err := cfg.ParseInto(&jc)
	if err != nil {
		log.Fatal("Module Config parsing failed")
	}
}

var confdata2 = `// start comment
{
"a" : "app",
// comment
"s" : {
   "b" : "x // y" // end line comment
  }
}`

func main() {

	// main application conf object
	cfg := &AppConfig{}

	buf := bytes.NewBufferString(confdata2)

	err := jconf.ParseInto(buf, &cfg)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Let our submodule parse its own config
	initSubModule(cfg.S)

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
}

```

Below is an example using only the subconfig feature:

```go
import (
	"github.com/One-com/gone/jconf"
	"encoding/json"
	"os"
	"bytes"
	"log"
)

type AppConfig struct {
	A string
	S *jconf.MandatorySubConfig
}

type ModuleConfig struct {
	B string
}

var confdata = `{ "a" : "app", "s" : {"b": "module"}}`

func initSubModule(cfg jconf.SubConfig) {
	var jc *ModuleConfig
	err := cfg.ParseInto(&jc)
	if err != nil {
		log.Fatal("Module Config parsing failed")
	}
}

func main() {
	// main application conf object
	cfg := &AppConfig{}

	// make a JSON decoder
	dec := json.NewDecoder(bytes.NewBuffer([]byte(confdata)))
	dec.UseNumber()

	// Parse the man config
	err := dec.Decode(&cfg)
	if err != nil {
		log.Fatal(err.Error())
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
}

```
