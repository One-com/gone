/*
Package daemon aims to make it easier to write simple network daemons for process supervised deployment like "systemd".

https://www.freedesktop.org/software/systemd/man/daemon.html

Specifically it supports the following:

   * Be able to implement graceful config reload without changing daemon PID.
     (A requirement for many other daemon supervisors, like "runit")
   * To not use the fork()/setsid()/fork() ritual to daemonize and not use "Type=forking"
     in the systemd unit file. (although you can)
   * Notify the init system about startup completion or status updates via the
     sd_notify(3) interface.
   * Socket activation for standard standard net package Listeners and Packetconns, with
     minimal code changes.
   * Using systemd FDSTORE to hold open filedescriptors during restart.

Package gone/daemon provides a master server manager for one or more services so they can be taken down and restarted while keeping any network connection or other file descriptors open. Several different reload/restart schemes are possible - see the examples.

Library layers

At the lower level the "gone/daemon/srv" package provides a "Server" interface and functions Serve(), Shutdown(). You can use this interface directly, implementing your own reload policy or use the higherlevel Run() function of the "gone/daemon" package.

At the higher level package "gone/daemon" provide the Run() function which takes a set of RunOptions, one of which must be a function to instantiate a slice of srv.Server objects (and possible define cleanup function) and serve these servers until Exit() is called while obeying Reload().

*/
package daemon
