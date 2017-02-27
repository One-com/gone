# gone daemon ctrl example

Package gone/daemon/crtl creates general UNIX domain socket server which can execute commands inside the daemon
process by implementing a simple Command interface.

Commands can be long-running (as outputting accesslog) and client connections to the socket will
survive daemon reload and re-spawning.

This has all kinds of potential usage. Inspecting and/or manipulating state, tweaking log levels and in general
instrument the daemon in ways not possible with simple OS signaling.

The program in this example shows how to use if for simple control of an HTTP server by changing Handler
state and outputting access log.

Start the server like:

``` shell
./example -s cmd.sock
```

It'll listen with HTTP on localhost:4321

You can connect to the socket with simple command line tools:

``` shell
rlwrap nc -U cmd.sock
```

or feed it commands directly:

``` shell
cat <(echo accesslog main) - | nc -U cmd.sock
```

The ctrl package is still somewhat experimental in terms of concept and API.

