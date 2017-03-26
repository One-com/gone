package main

import (
	"fmt"
	"github.com/One-com/gone/daemon"
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
		Server: s1,
		Listeners: daemon.ListenerGroup{daemon.ListenerSpec{Addr: ":4321"}},
	}
	// Now a gone/http/goneserv.Server, expecting to be called upon to Listen()
	return s3
}

func loadConfig(cfg string) daemon.ConfigFunc {
	var revision int
	cf := daemon.ConfigFunc(
		func() (s []daemon.Server, c []daemon.CleanupFunc, err error) {
			revision++
			log.Printf("Loading config. rev: %d", revision)

			s = make([]daemon.Server, 1)
			c = make([]daemon.CleanupFunc, 1)

			s[0] = newHTTPServer(http.HandlerFunc(myHandlerFunc(cfg, revision)))

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

	signals.RunSignalHandler(handledSignals)
}

//---------------------------------------------------------------------------

func main() {

	configureFunc := loadConfig("myconf")

	log.Println("Starting server", "PID", os.Getpid())

	runoptions := []daemon.RunOption{
		daemon.Configurator(configureFunc),
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
