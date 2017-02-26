package ctrl

import (
	"fmt"
	"net"
	"bufio"
	"os"
	"bytes"
	"io"
	"io/ioutil"
	"github.com/One-com/gone/sd"
	"github.com/One-com/gone/daemon/srv"
	"sync"
	"context"
	"strings"
	"path"
	//"encoding/hex"
	unix "syscall"
)

// Command is the interface of a specific command
type Command interface {
	// ShotUsage provides a short description of command argument syntax and possibly a comment.
	// to be printed a listing of commands.
	ShortUsage() (syntax string, comment string)

	// Usage let the command provide it's own full documentation, being passed the command name
	// the command is registered under.
	Usage(cmd string, out io.Writer)

	// Invoke invokes the command being passed a context an io.Writer to the socket
	// the command name used to invoke it and it's arguments.
	// Invoke has the option of returning function to be invoked
	// asynchronously and - optionally command line to potentially make it persistent.
	// The persistent command will be re-executed
	// after a reload - whether or not that is in a new process
	Invoke(ctx context.Context, conn io.Writer, cmd string, args []string) (async func(), persistent string, err error)
}

var (
	cmdmu       sync.Mutex
	commands    map[string]Command
)


func init() {
	commands = make(map[string]Command)
}

// RegisterCommand registers an implementation of the Command interface under a command name
func RegisterCommand(name string, cmd Command) {
	cmdmu.Lock()
	defer cmdmu.Unlock()
	commands[name] = cmd
}

// a connection and the cmdline it is currently executing
type persistentConn struct {
	net.Conn
	cmdline []byte
}

// Server implements a server accepting connections on a UNIX domain socket on which
// registered commands can be invoked.
// The client connections to this socket will survive process Reload and Replacement.
type Server struct {

	// Path on which the server will listen.
	Addr string
	// Systemd socket name. If no Addr is given the socket can be provided
	// via systemd socket activation.
	ListenerFdName string

	// The command invoking the help system
	HelpCommand string
	// The command to cause the server to close a connection.
	QuitCommand string

	l  net.Listener

	wg sync.WaitGroup
 	// connections which need to be served
 	conns []persistentConn

	mu sync.Mutex
	doneChan chan struct{}

	ctx       context.Context
	ctxCancel context.CancelFunc

	// A logger to log errors during client connections to.
	Logger srv.LoggerFunc
}


func (s *Server) getDoneChan() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getDoneChanLocked()
}



func (s *Server) getDoneChanLocked() chan struct{} {

	if s.doneChan == nil {
		s.doneChan = make(chan struct{})

	}

	return s.doneChan
}


func (s *Server) closeDoneChanLocked() {

	ch := s.getDoneChanLocked()

	select {
	case <-ch:
		// Already closed. Don't close again.
	default:
		close(ch)
	}
}

// Description implement gone/daemon/srv.Descripter interface.
func (s *Server) Description() string {
        return fmt.Sprintf("CMD socket(%s)", s.ListenerFdName)
}

// Listen implement the gone/daemon/srv.Listener interface and
// pick an already open listener FD or create one.
func (s *Server) Listen() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ctx, s.ctxCancel = context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			s.ctxCancel()
		}
	}()

	s.conns = nil
	// Restore any open connections
	//var count int
	var names []string
	_, names, err = sd.ListenFdsWithNames()
	if err != nil {
		return
	}
	for _, fdname := range names {
		if strings.HasPrefix(fdname, "gonectrl") {
			connname := strings.TrimPrefix(fdname, "gonectrl")
			gonecmd := "gonecmd" + connname
			//fmt.Fprintln(os.Stderr, "I:", fdname)
			//fmt.Fprintln(os.Stderr, "I:", gonecmd)
			var file, cmdfile *os.File
			file,_, err = sd.FileWith(fdname)
			if err != nil {
				return
			}
			var conn net.Conn
			conn, err = net.FileConn(file)
			if err != nil {
				return
			}
			file.Close() // FileConn() did a dup - we don't need this one now.
			cmdfile, _, err = sd.FileWith(gonecmd)
			if err != nil {
				return
			}
			cmdfile.Seek(0,0)
			var cmdline []byte
			cmdline, err = ioutil.ReadAll(cmdfile)
			if err != nil {
				return
			}
			active := persistentConn{Conn: conn, cmdline: cmdline}
			s.conns = append(s.conns, active)
			cmdfile.Close() // not needed any more - we have the cmdline
		}
	}

	// Need to start serving inherited persistent control connections
	// Now since we cannot guarantee that the commands will be persisted/re-invoked.
	// Before other servers start Serve().
	// First serve any persistent connections
	for _, active := range s.conns {
		s.wg.Add(1)
		go s.serve(s.ctx, active.Conn, active.cmdline)
	}

	// Now listen for our own connections
	var uaddr *net.UnixAddr

	uaddr, err = net.ResolveUnixAddr("unix", s.Addr)
	if err != nil {
		return
	}

	s.l, err = sd.NamedListenUnix(s.ListenerFdName, "unix", uaddr)
	return
}

// Serve implement the gone/daemon/srv.Server interface.
// It is synchronous (does not implement Wait). All connections
// should exit ASAP when Shutdown is called.
func (s *Server) Serve() (err error) {
	defer s.l.Close()

	defer s.wg.Wait()

	var ctx context.Context
	s.mu.Lock()
	ctx = s.ctx
	defer s.ctxCancel()
	s.mu.Unlock()

	for {
		conn, e := s.l.Accept()
		if e != nil {
			select {
			case <-s.getDoneChan():
				return nil

			default:
				return e
			}
		}

		s.wg.Add(1)
		go s.serve(ctx, conn, nil)
	}
}

func (s *Server) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// mark the server shut down
	s.closeDoneChanLocked()
	// make it stop
	s.l.Close()
	// cancel the serving context to make running connections stop
	s.ctxCancel()
}

func (s *Server) serve(pctx context.Context, c net.Conn, initialcmd []byte) {

	defer c.Close()
	defer s.wg.Done()

	var cmdwg = &sync.WaitGroup{}

	var ctx context.Context       // currently executed command context.
	var cancel context.CancelFunc // function to call to cancel currently executing command.

	var quitCommand = s.QuitCommand
	//if quitCommand == "" {
	//	quitCommand = "quit"
	//}
	var helpCommand = s.HelpCommand
	//if helpCommand == "" {
	//	helpCommand = "help"
	//}


	// An anonymous inode file which contains the current executing commandline
	var err error
	var cmdfile *os.File
	cmdfile, err = ioutil.TempFile("", "gonectrl")
	if err != nil {
		fmt.Fprintln(c, "Unable to persist command. No tmpfile: " + err.Error())
		return
	}

	// the sd names of the fds passed to next Server instance.
	gonectrl := path.Base(cmdfile.Name())
	gonecmd  := "gonecmd" + strings.TrimPrefix(gonectrl, "gonectrl")

	err = unix.Unlink(cmdfile.Name())
	if err != nil {
		fmt.Fprintln(c, "Unable to persist command. No unlink: " + err.Error())
		return
	}
	defer cmdfile.Close()

	// Export the connection socket to make it persistent across reload/respawn/restart
	// Export an fd for the cmdfile inode too.
	// We can't remove exports, so if any later command is not
	// persistent, then the next server will notice that the
	// stored cmdfile is empty and not re-invoke the command.
	err = sd.Export(gonectrl, c) // TODO: we're relying on conn actually implementing File()
	if err != nil {
		if s.Logger != nil {
			s.Logger(srv.LvlCRIT, fmt.Sprintf("Failed to export control socket conn: %s", err.Error()))
		}
		return
	}
	err = sd.Export(gonecmd, cmdfile)
	if err != nil {
		if s.Logger != nil {
			s.Logger(srv.LvlCRIT, fmt.Sprintf("Failed to export control socket cmd: %s", err.Error()))
		}
		return
	}

	// Be sure to have server shutdown stop the scanner.
	stopch := make(chan struct{})
	defer close(stopch)
	go func () {
	STOPLOOP:
		for {
			// exit when server(conn) exits
			// but force that to happen when the Serve() context is canceled.
			select {
			case <-pctx.Done():
				c.Close()
			case <-stopch:
				break STOPLOOP
			}
		}
	}()

	// Read connection as lines, executing commands.
	// Start with any initial command provided, then parse lines from the connection
	scanner := bufio.NewScanner(c)
	for initialcmd != nil || scanner.Scan() { // Lazy eval. First run initial
		var line []byte
		if initialcmd != nil {
			line = initialcmd
			initialcmd = nil
		} else {
			line = scanner.Bytes()
		}

		lscanner := bufio.NewScanner(bytes.NewReader(line))
		// Set the split function for the scanning operation.
		lscanner.Split(bufio.ScanWords)

		// Tokenize the line to be able to let flag Parse it
		var tokens []string
		for lscanner.Scan() {
			tokens = append(tokens, lscanner.Text())
		}
		if err := lscanner.Err(); err != nil {
			if s.Logger != nil {
				s.Logger(srv.LvlERROR, fmt.Sprintf("reading line: %s", err.Error()))
			}
		}

		// Empty line, - read next line.
		if len(tokens) == 0 {
			continue
		}


		// Since the current running command is now know to be replaced.
		// remove it from persistent file.
		cmdfile.Truncate(0)
		cmdfile.Seek(0,0)
		cmdfile.Sync()

		// ctx can be nil if last cmd was a help command.
		// help commands don't have a context.
		// If there a running command, cancel it.
		if ctx != nil {
			cancel()
			<-ctx.Done()
			ctx = nil // remove the command context
			// wait for the previous command to exit
			cmdwg.Wait()
		}

		var cmdhelp bool // if true, don't run current command. Show its help instead.
		var cmd string   // current command

		// TODO
		if tokens[0] == quitCommand {
			sd.Forget(gonectrl)
			sd.Forget(gonecmd)
			cmdwg.Wait() // Don't exit until the current executing command has gotten the message.
			return
		}

		// Handle any help commands
		if helpCommand != "" && tokens[0] == helpCommand {
			if len(tokens) == 2 {
				cmd = tokens[1]
				cmdhelp = true
			} else {
				s.help(c, helpCommand, quitCommand)
				continue
			}
		} else {
			cmd = tokens[0]
		}

		// Get the current command.
		cmdmu.Lock()
		cmdobj, ok := commands[cmd]
		cmdmu.Unlock()

		// Either invoke it, or invoke it's help
		if ok {
			if cmdhelp {
				cmdobj.Usage(cmd, c)
			} else {
				ctx, cancel = context.WithCancel(pctx)
				async, persistent, err := cmdobj.Invoke(ctx, c, cmd, tokens[1:])
				if err == nil {
					if async != nil {
						if persistent != "" {
						// record the command for the next server
						// so it can be replayed.
							cmdfile.WriteString(persistent)
							cmdfile.Sync()
						}
						// Invoke the command in it's own go routine to be able to still read a next command on the conn
						cmdwg.Add(1)
						go func() {
							defer cmdwg.Done()
							async()  // Invoke the command.
							cancel() // make sure it's cancel'ed if it exits by it self.
						}()
					} else {
						cancel()
					}

				} else {
					fmt.Fprintln(c, "Error: ", err.Error())
				}
			}
		} else {
			if helpCommand != "" {
				fmt.Fprintln(c, "Unknown command, try: " + helpCommand)
			} else {
				fmt.Fprintln(c, "Unknown command")
			}
		}

		// Ready for the next command - loop to the scanner.
	}

	if err := scanner.Err(); err != nil {
		if s.Logger != nil {
			s.Logger(srv.LvlWARN, fmt.Sprintf("reading connection: %s", err.Error()))
		}
	}

	// Cancel any still pending command
	if ctx != nil {
		// Then cancel the current invocation go-routine to be sure it's done.
		cancel()
	}

	cmdwg.Wait() // Don't exit until the current executing command has gotten the message.

	// If the scanner exited by itself, close the conn for good
	select {
	case <-pctx.Done():
	default:
		// There was an EOF on the connection. Close it for good
		sd.Forget(c)
		sd.Forget(cmdfile)
	}
}

type usageinfo struct {
	syntax string
	comment string
}

func (s *Server) help(w io.Writer, hcmd, qcmd string) {

	cmdmu.Lock()
	defer cmdmu.Unlock()

	var cmdlength, syntaxlength, commentlength int
	var _usageinfo = make(map[string]*usageinfo)
	for cmd  := range commands {
		cmdobj, ok := commands[cmd]

		if len(cmd) > cmdlength {
			cmdlength = len(cmd)
		}

		if ok {
			syntax, comment := cmdobj.ShortUsage()
			_usageinfo[cmd] = &usageinfo{syntax, comment}
			if len(syntax) > syntaxlength {
				syntaxlength = len(syntax)
			}
			if len(comment) > commentlength {
				commentlength = len(comment)
			}
		} else {
			_usageinfo[cmd] = nil
		}
	}


	fmt.Fprintln(w, "---- commands --------------------------------------------------------------")
	if qcmd != "" {
		fmt.Fprintf(w, "%-*s %-*s - %-*s\n", cmdlength, qcmd, syntaxlength, "", commentlength, "exit and close the connection")
	}
	if hcmd != "" {
		fmt.Fprintf(w, "%-*s %-*s - %-*s\n", cmdlength, hcmd, syntaxlength, "", commentlength, "help")
	}


	for cmd, info  := range _usageinfo {
		if info != nil {
			fmt.Fprintf(w, "%-*s %-*s - %-*s\n", cmdlength, cmd, syntaxlength, info.syntax, commentlength, info.comment)
		} else {
			fmt.Fprintf(w, "%s - ERROR\n", cmd)
		}
	}
}
