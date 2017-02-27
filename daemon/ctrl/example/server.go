package main

import (
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"os"
	"time"
	"sync"
	"context"

	"github.com/One-com/gone/daemon/srv"
	"github.com/One-com/gone/daemon/ctrl"

	"github.com/One-com/gone/http/gonesrv"
	"github.com/One-com/gone/http/graceful"
	"github.com/One-com/gone/http/handlers/accesslog"
	"github.com/One-com/gone/log"
	"github.com/One-com/gone/log/syslog"

)

func myHandlerFunc(s *Server, cfg string, revision int) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		curval := s.GetValue()
		io.WriteString(w, fmt.Sprintf("I'm here. state: \"%s\", cfg: %s, rev: %d, pid %d\n", curval, cfg, revision, os.Getpid()))
	})
}


//----------------- The actual HTTP server ----------------------
// maintaining a simple string as state.

type Server struct {
	*gonesrv.Server
	mu sync.Mutex
	curval string
}

func (s *Server) GetValue() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.curval
}

func (s *Server) SetValue(val string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.curval = val
}

func newHTTPServer(cfg string, rev int) (s srv.Server) {

	gonelog := log.NewStdlibAdapter(log.Default(), syslog.LOG_CRIT)
	errorlog := stdlog.New(gonelog, "", stdlog.LstdFlags)

	// basic HTTP server
	s1 := &http.Server{
		Addr:     ":4321",
		ErrorLog: errorlog,
	}
	// wrapped to get Shutdown() and graceful shutdown
	s2 := &graceful.Server{
		Server:  s1,
		Timeout: time.Duration(20) * time.Second,
	}
	// wrapped to get Listen()
	s3 := &gonesrv.Server{
		Server: s2,
	}
	s4 := &Server{
		Server: s3,
	}

	srvctrl := newServerControlCommand(s4)

	ctrl.RegisterCommand("value", srvctrl)

	accesslogHandler := accesslog.NewDynamicLogHandler(myHandlerFunc(s4, cfg, rev), nil)

	accessLogControl.RegisterLogHandler("main", accesslogHandler)

	s1.Handler = accesslogHandler

	// Now a gone/http/goneserv.Server, expecting to be called upon to Listen()
	return s4
}

// -- the command controlling server state

type serverControlCommand struct {
	s *Server
}

func (sc *serverControlCommand) Invoke(ctx context.Context, w io.Writer, cmd string, args []string) (async func(), persistent string, err error ) {
	if len(args) > 0 {
		switch args[0] {
		case "set":
			var val string
			if len(args) == 2 {
				val = args[1]
			} else {
				val = ""
			}
			sc.s.SetValue(val)
		case "get":
			val := sc.s.GetValue()
			fmt.Fprintln(w, val)
		}
	}
	return
}

func (sc *serverControlCommand) ShortUsage() (syntax, comment string) {
	syntax  = "[get | set <value> ]"
	comment = "Control server state"
	return
}

func (sc *serverControlCommand) Usage(cmd string, w io.Writer) {
	fmt.Fprintln(w, cmd, " - set or get a server state value")
	fmt.Fprintln(w, "  ", cmd, "get")
	fmt.Fprintln(w, "  ", cmd, "set <value>")
}

func newServerControlCommand(s *Server) ctrl.Command {
	return &serverControlCommand{s}
}
