# gone daemon ctrl

Package gone/daemon/crtl creates general UNIX domain socket server which can execute commands inside the daemon
process by implementing a simple Command interface.

Commands can be long-running (as outputting accesslog) and client connections to the socket will
survive daemon reload and re-spawning.

This has all kinds of potential usage. Inspecting and/or manipulating state, tweaking log levels and in general
instrument the daemon in ways not possible with simple OS signaling.

You can connect to the socket with simple command line tools:

``` shell
rlwrap nc -U /var/run/mydaemon/cmd.sock
```

or feed it commands directly:

``` shell
cat <(echo accesslog) - | nc -U /var/run/mydaemon/cmd.sock
```

The ctrl package is still somewhat experimental in terms of concept and API.

