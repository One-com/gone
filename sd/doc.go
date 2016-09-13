/*
Package gone/sd aims to make it easier to write simple network daemons for Linux systemd

https://www.freedesktop.org/software/systemd/man/daemon.html

Specifically it supports the following:

   * Socket activation for standard standard net package Listeners and Packetconns, with 
     minimal code changes.
   * Notify the init system about startup completion or status updates via the
     sd_notify(3) interface.
   * Using systemd FDSTORE to hold open file descriptors during restart.
   * Systemd watchdog support

Package "sd" is not depended on systemd as such. If there's no socket activation available the fallback is most often
to just create the socket. If there's no notifiy socket, calling sd.Notify() will of course fail.

*/
package sd
