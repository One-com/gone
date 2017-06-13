package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/One-com/gone/daemon"
	"github.com/One-com/gone/daemon/ctrl"
	"github.com/One-com/gone/daemon/srv"
	"github.com/One-com/gone/log"
	"github.com/One-com/gone/log/syslog"
	"github.com/One-com/gone/sd"
	"github.com/One-com/gone/signals"
	"io"
	"os"
	"syscall"
)

var accessLogControl = newAccessLogCommand(serverLogFunc)
var procControl = &procCommand{}

func loadConfig(cfg string) daemon.ConfigureFunc {
	var revision int
	cf := daemon.ConfigureFunc(
		func() (s []srv.Server, c []daemon.CleanupFunc, err error) {
			revision++
			log.Printf("Loading config. rev: %d", revision)

			s = make([]srv.Server, 1)
			c = make([]daemon.CleanupFunc, 1)

			accessLogControl.Reset()

			s[0] = newHTTPServer(cfg, revision)
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

//---------------------------------------------------------

var controlSocket string

func init() {

	flag.StringVar(&controlSocket, "s", "", "Path to control socket")

	flag.Parse()

	ctrl.RegisterCommand("accesslog", accessLogControl)
	ctrl.RegisterCommand("proc", procControl)

	/* Setup signalling */

	handledSignals := signals.Mappings{
		syscall.SIGINT:  onSignalExit,
		syscall.SIGTERM: onSignalExitGraceful,
		syscall.SIGHUP:  onSignalReload,
		syscall.SIGUSR2: onSignalRespawn,
		syscall.SIGTTIN: onSignalIncLogLevel,
		syscall.SIGTTOU: onSignalDecLogLevel,
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
		daemon.ControlSocket("", controlSocket),
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

// A simple command doing the same as the registered OS signals

type procCommand struct{}

func (p *procCommand) ShortUsage() (syntax, comment string) {
	syntax = "[reload|stop|respawn]"
	comment = "control the daemon process"
	return
}

func (p *procCommand) Usage(cmd string, w io.Writer) {
	fmt.Fprintln(w, cmd, "control the process")
}

func (p *procCommand) Invoke(ctx context.Context, w io.Writer, cmd string, args []string) (async func(), persistent string, err error) {
	cmd = args[0]
	switch cmd {
	case "reload":
		onSignalReload()
	case "stop":
		onSignalExit()
	case "respawn":
		onSignalRespawn()
	default:
		fmt.Fprintln(w, "Unknown action")
	}
	return
}
