package main

import (
	"fmt"
	"github.com/One-com/gone/daemon"
	"github.com/One-com/gone/daemon/ctrl"
	"github.com/One-com/gone/signals"
	gonehttp "github.com/One-com/gone/http"
	"github.com/One-com/gone/log"
	"github.com/One-com/gone/log/syslog"
	"github.com/One-com/gone/sd"
	"io"
	stdlog "log"
	"net/http"
	"os"
	"syscall"
	"time"
	"context"
	"strconv"
)

//----------------- The actual server ----------------------

func myHandlerFunc(cfg string, revision int) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, fmt.Sprintf("I'm here. cfg: %s, rev: %d, pid %d\n", cfg, revision, os.Getpid()))
	})
}

func newHTTPServer(handler http.HandlerFunc) (s *gonehttp.Server) {

	gonelog := log.NewStdlibAdapter(log.Default(), syslog.LOG_CRIT)
	errorlog := stdlog.New(gonelog, "", stdlog.LstdFlags)

	// basic HTTP server
	s1 := &http.Server{
		//Addr:     ":4321",
		Handler:  handler,
		ErrorLog: errorlog,
	}

	s3 := &gonehttp.Server{
		Name: "Example",
		Server: s1,
		Listeners: daemon.ListenerGroup{daemon.ListenerSpec{Addr: ":4321"}},
	}
	// Now a gone/http/goneserv.Server, expecting to be called upon to Listen()
	return s3
}

type trivialServer struct {}
func (t *trivialServer) Serve(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
LOOP:
	for {
		select {
		case <-ticker.C:
			log.Println("trivial tick")
		case <-ctx.Done():
			break LOOP
		}
	}
	return nil
}

func (t *trivialServer) Description() string {
	return "Trivial"
}

func loadConfig(cfg string) daemon.ConfigFunc {
	var revision int
	cf := daemon.ConfigFunc(
		func() (s []daemon.Server, c []daemon.CleanupFunc, err error) {
			revision++
			log.Printf("Loading config. rev: %d", revision)

			s = make([]daemon.Server, 2)
			c = make([]daemon.CleanupFunc, 1)

			s[0] = newHTTPServer(http.HandlerFunc(myHandlerFunc(cfg, revision)))
			s[1] = &trivialServer{}

			localRevision := revision
			c[0] = func() error {
				log.Printf("Ran cleanup, rev: %d", localRevision)
				return nil
			}
			return
		})
	return cf
}

//----------------- Signal handling ----------------------


func onSignalExit() {
	log.Println("Signal Exit")
	daemon.Exit(false)
}

func onSignalExitGraceful() {
	log.Println("Signal Exit - graceful")
	sd.Notify(0, "STOPPING=1")
	daemon.Exit(true)
}

func onSignalReload() {
	log.Println("Signal Reload")
	sd.Notify(0, "RELOADING=1")
	daemon.Reload()
}

func onSignalRespawn() {
	log.Println("Signal Respawn")
	daemon.ReplaceProcess(syscall.SIGTERM)
}

func onSignalIncLogLevel() {
	log.IncLevel()
	log.ALERT(fmt.Sprintf("Log level: %d", log.Level()))
}

func onSignalDecLogLevel() {
	log.DecLevel()
	log.ALERT(fmt.Sprintf("Log level: %d\n", log.Level()))
}

func serverLogFunc(level int, message string) {
	log.Log(syslog.Priority(level), message)
}

var procControl      = &procCommand{}

func init() {

	/* Setup signalling */

	handledSignals := signals.Mappings{
		syscall.SIGINT  : onSignalExit,
		syscall.SIGTERM : onSignalExitGraceful,
		syscall.SIGHUP  : onSignalReload,
		syscall.SIGUSR2 : onSignalRespawn,
		syscall.SIGTTIN : onSignalIncLogLevel,
		syscall.SIGTTOU : onSignalDecLogLevel,
	}

	log.SetLevel(syslog.LOG_DEBUG)
	daemon.SetLogger(serverLogFunc)

	ctrl.RegisterCommand("daemon", procControl)

	signals.RunSignalHandler(handledSignals)
}

//---------------------------------------------------------------------------

func main() {

	configureFunc := loadConfig("myconf")

	log.Println("Starting server", "PID", os.Getpid())

	runoptions := []daemon.RunOption{
		daemon.Configurator(configureFunc),
		daemon.ControlSocket("", "ctrl.sock"),
		daemon.ShutdownTimeout(time.Duration(4*time.Second)),
		daemon.SdNotifyOnReady(true, "Ready and serving"),
		daemon.SignalParentOnReady(),
	}

	err := daemon.Run(runoptions...)
	if err != nil {
		log.Println(err)
	}

	sd.Notify(sd.NotifyUnsetEnv, "STATUS=Terminated")
	log.Println("Halted")
}

// ---------------------------------------------------------------
// A simple control socket command controlling the daemon process

type procCommand struct {}

func (p *procCommand) ShortUsage() (syntax, comment string) {
	syntax = "[reload|respawn|kill|stop <timeout seconds>]"
	comment = "control the daemon process"
	return
}

func (p *procCommand) Usage(cmd string, w io.Writer) {
	fmt.Fprintln(w, cmd, "control the process")
}

func (p *procCommand) Invoke(ctx context.Context, w io.Writer, cmd string, args []string) (async func(), persistent string, err error ) {
	cmd = args[0]
	switch cmd {
	case "reload":
		onSignalReload()
	case "kill":
		onSignalExit()
	case "stop":
		var timeout time.Duration
		if args[1] != "" {
			var to int
			to, err = strconv.Atoi(args[1])
			if err != nil {
				return
			}
			timeout = time.Second * time.Duration(to)
		}
		log.Printf("Signal Exit - graceful: %s", timeout.String())
		sd.Notify(0, "STOPPING=1")
		daemon.ExitGracefulWithTimeout(timeout)
	case "respawn":
		onSignalRespawn()
	default:
		fmt.Fprintln(w, "Unknown action")
	}
	return
}
