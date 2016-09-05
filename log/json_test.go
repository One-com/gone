package log_test

import (
	"testing"
	"bytes"
	"encoding/json"
	"time"
	"github.com/One-com/gone/log"
	"github.com/One-com/gone/log/syslog"
)


var keys = &log.EventKeyNames{
	Lvl:  "lvl",
	Name: "name",
	Time: "ts",
	Msg:  "msg",
	File: "file",
	Line: "line",
}

func TestJSON(t *testing.T) {

	var b bytes.Buffer
	var layout string = "Mon Jan 2 15:04:05 -0700 MST 2006"
	
	opt1 := log.KeyNamesOpt(keys)
	opt2 := log.TimeFormatOpt(layout)
	h := log.NewJSONFormatter(&b, opt1, opt2)
	l := log.GetLogger("json")
	l.SetPrintLevel(syslog.LOG_ERROR, false)
	l.SetHandler(h)

	now := time.Now()

	l2 := l.With("foo","bar")
	l2.Println("hello")
	output := b.String()
	var result map[string]interface{}
	err := json.Unmarshal([]byte(output),&result)
	if err != nil {
		t.Errorf("Invalid JSON result: (%s) %s", err.Error(), output)
	}
	if syslog.Priority(result["lvl"].(float64)) != syslog.LOG_ERROR ||
		result["name"].(string) != "json" ||
		result["msg"].(string) != "hello" ||
		result["foo"].(string) != "bar" {
		t.Errorf("Missing JSON fields in: %s", output)
	}
	timestr := result["ts"].(string)

	tres, _ := time.Parse(layout, timestr)
	if tres.Sub(now).Seconds() > 0.001 {
		t.Errorf("Timestamp didn't survive JSON: %s", timestr)
	}
}
