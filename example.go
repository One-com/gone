package main

import (
	"fmt"
	"github.com/One-com/gone/daemon"
	"github.com/One-com/gone/daemon/srv"
	"github.com/One-com/gone/http/gonesrv"
	"github.com/One-com/gone/http/graceful"
	"github.com/One-com/gone/log"
	"github.com/One-com/gone/sd"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//----------------- The actual server ----------------------

func myHandlerFunc(cfg string, revision int) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, fmt.Sprintf("I'm here. cfg: %s, rev: %d, pid %d\n", cfg, revision, os.Getpid()))
	})
}

func newHTTPServer(handler http.HandlerFunc) (s srv.Server) {
	// basic HTTP server
	s1 := &http.Server{
		Addr:    ":4321",
		Handler: handler,
		//		ErrorLog: log.New(os.Stdout,"",log.LstdFlags),
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
	// Now a gone/http/goneserv.Server, expecting to be called upon to Listen()
	return s3
}

func loadConfig(cfg string) daemon.ConfigureFunc {
	var revision int
	cf := daemon.ConfigureFunc(
		func() (s []srv.Server, c []daemon.CleanupFunc, err error) {
			log.Println("Loading config")

			s = make([]srv.Server, 1)
			c = make([]daemon.CleanupFunc, 1)
			revision++
			s[0] = newHTTPServer(http.HandlerFunc(myHandlerFunc(cfg, revision)))
			c[0] = func() error {
				log.Println("Ran cleanup")
				return nil
			}
			return
		})
	return cf
}

//----------------- Signal handling ----------------------

type signalAction func()

var (
	sigch         chan os.Signal
	signalActions map[os.Signal]signalAction
)

func signalHandler(sigch chan os.Signal, actions map[os.Signal]signalAction) {
	for {
		select {
		case s := <-sigch:
			f := actions[s]
			f()
		}
	}
}

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

func init() {

	/* Setup signalling */
	sigch = make(chan os.Signal)
	signalActions = make(map[os.Signal]signalAction)

	signalActions[syscall.SIGINT] = onSignalExit
	signalActions[syscall.SIGTERM] = onSignalExitGraceful
	signalActions[syscall.SIGHUP] = onSignalReload
	signalActions[syscall.SIGUSR2] = onSignalRespawn
	signalActions[syscall.SIGTTIN] = onSignalIncLogLevel
	signalActions[syscall.SIGTTOU] = onSignalDecLogLevel

	for sig := range signalActions {
		signal.Notify(sigch, sig)
	}
}

//---------------------------------------------------------------------------

func main() {

	go signalHandler(sigch, signalActions)

	configureFunc := loadConfig("myconf")

	log.Println("Starting server", "PID", os.Getpid())

	runoptions := []daemon.RunOption{
		daemon.InstantiateServers(configureFunc),
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
