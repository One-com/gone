package main

import (
	"fmt"
	"io"
	"sync"
	"context"
	"flag"
	"strings"
	"github.com/One-com/gone/daemon/srv"
	"github.com/One-com/gone/http/handlers/accesslog"
)

// A command turning on/off accesslog for registered HTTP handlers.

type accessLogCommand struct {
	mu sync.Mutex
	handlers map[string]accesslog.DynamicLogHandler
	logger srv.LoggerFunc
}

func (lc *accessLogCommand) Reset() {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.handlers = make(map[string]accesslog.DynamicLogHandler)
}


func (lc *accessLogCommand) RegisterLogHandler(name string, lh accesslog.DynamicLogHandler) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.handlers[name] = lh
}

func (lc *accessLogCommand) ShortUsage() (syntax, comment string) {
	syntax  = "-list | <handler>"
	comment = "Output accesslog"
	return
}

func (lc *accessLogCommand) Usage(cmd string, w io.Writer) {
	fmt.Fprintln(w, cmd, "-list       List available handlers")
	fmt.Fprintln(w, cmd, "<handler>   Output access log for this handler")
}

func (lc *accessLogCommand) Invoke(ctx context.Context, w io.Writer, cmd string, args []string) (async func(), persistent string, err error ) {

	fs := flag.NewFlagSet("alog", flag.ContinueOnError)
	list := fs.Bool("list", false, "List HTTP handlers capable of access log")
	fs.SetOutput(w)
	err = fs.Parse(args)
	if err != nil {
		fmt.Fprintf(w,"Syntax error: %s", err.Error())
		return
	}

	if *list && fs.NArg() == 0 {
		lc.mu.Lock()
		for handler := range lc.handlers {
			fmt.Fprintln(w, handler)
		}
		lc.mu.Unlock()
		return
	}

	args = fs.Args()

	var hname string
	if len(args) == 1 {
		hname = args[0]
	}
	lc.mu.Lock()
	handler, ok := lc.handlers[hname]
	lc.mu.Unlock()
	if !ok {
		fmt.Fprintln(w,"No logging")
		return
	}

	argstr := strings.Join(args, " ")
	persistent = strings.Join([]string{cmd, argstr}, " ")

	async = func() {
		lc.logger(srv.LvlINFO,"Turning on accesslog")
		handler.ToggleAccessLog(nil, w)
		<-ctx.Done()
		lc.logger(srv.LvlINFO,"Turning off accesslog")
		handler.ToggleAccessLog(w, nil)
	}
	return
}


func newAccessLogCommand(logger srv.LoggerFunc) *accessLogCommand {
	lc := &accessLogCommand{logger: logger}
	lc.Reset()
	return lc
}
