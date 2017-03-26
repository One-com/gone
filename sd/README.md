# gone/sd

Golang systemd library [![GoDoc](https://godoc.org/github.com/one-com/gone/sd?status.svg)](https://godoc.org/github.com/one-com/gone/sd)

Package gone/sd provides tools for having daemons interact with the Linux systemd init system for socket activation and notification.

This library aims to make it easy to write graceful daemons for Linux Systemd services
in the "newstyle" way:

http://www.freedesktop.org/software/systemd/man/daemon.html

## Overview

Package gone/sd consists of two parts.

* Communicating over the systemd notify socket to inform systemd about process status and send file descriptors to the systemd FDSTORE

* Replacement functions for some of the stdlib "net" package for parsing the environment passed on from systemd to create sockets (and other files) inherited from systemd.

