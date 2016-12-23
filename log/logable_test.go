package log_test

import (
	"bytes"
	"fmt"
	"github.com/One-com/gone/log"
)

type MyObj struct {
	Key1 string
	Key2 string
}

func (o *MyObj) LogValues() log.KeyValues {
	return []interface{}{"key1", o.Key1, "key2", o.Key2}
}

func ExampleLogable() {

	obj := &MyObj{"foo","bar"}

	var b bytes.Buffer
	l := log.New(&b, "", 0)
	l.With(log.KV{"orange": "apple"}, "a", "b").ERROR("An", obj, "pif", "paf")

	fmt.Println(b.String())
	// Output:
	// An orange=apple a=b key1=foo key2=bar pif=paf
}
