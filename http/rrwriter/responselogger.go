// This file contains slightly modified code from the Gorilla project.
//
// Copyright 2013 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Source:
// https://github.com/gorilla/handlers/blob/master/handlers.go

package rrwriter

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"time"
)

// MakeRecorder returns a RecordingResponseWriter which wraps around the
// provided http.ResponseWriter.
// After the response has been written, you can get Size() and Status()
func MakeRecorder(w http.ResponseWriter) RecordingResponseWriter {
	var logger RecordingResponseWriter = &responseRecorder{w: w}
	if _, ok := w.(http.Hijacker); ok {
		logger = &hijackResponseRecorder{responseRecorder{w: w}}
	}
	h, ok1 := logger.(http.Hijacker)
	c, ok2 := w.(http.CloseNotifier)
	if ok1 && ok2 {
		return hijackCloseNotifier{logger, h, c}
	}
	if ok2 {
		return &closeNotifyWriter{logger, c}
	}
	return logger
}

// RecordingResponseWriter is the interface of the recorder.
type RecordingResponseWriter interface {
	http.ResponseWriter
	http.Flusher
	Status() int
	Size() int
	GetTimeStamp() time.Time
	SetTimeStamp(time.Time)
}

// responseRecorder is wrapper of http.ResponseWriter that keeps track of its HTTP
// status code and body size
type responseRecorder struct {
	w      http.ResponseWriter
	status int
	size   int
	ts     time.Time
}

// Header implements http.ResponseWriter
func (l *responseRecorder) Header() http.Header {
	return l.w.Header()
}

// Write implements http.ResponseWriter
func (l *responseRecorder) Write(b []byte) (int, error) {
	if l.status == 0 {
		// The status will be StatusOK if WriteHeader has not been called yet
		l.status = http.StatusOK
	}
	size, err := l.w.Write(b)
	l.size += size
	return size, err
}

// WriteHeader implements http.ResponseWriter
func (l *responseRecorder) WriteHeader(s int) {
	l.w.WriteHeader(s)
	l.status = s
}

// GetTimeStamp returns the timestamp
// This can be used to store a starttime for a request and return it
// after the request is done.
// It is not go-routine safe.
func (l *responseRecorder) GetTimeStamp() (tout time.Time) {
	tout = l.ts
	return
}

// SetTimeStamp sets a timestamp.
// This can be used to store a starttime for a request and return it
// after the request is done.
// It is not go-routine safe.
func (l *responseRecorder) SetTimeStamp(tin time.Time) {
	l.ts = tin
}


// Status returns the http status of the written request.
func (l *responseRecorder) Status() int {
	return l.status
}

// Size returns the written response size.
func (l *responseRecorder) Size() int {
	return l.size
}

func (l *responseRecorder) Flush() {
	f, ok := l.w.(http.Flusher)
	if ok {
		f.Flush()
	}
}

type hijackResponseRecorder struct {
	responseRecorder
}

func (l *hijackResponseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h := l.responseRecorder.w.(http.Hijacker)
	conn, rw, err := h.Hijack()
	if err == nil && l.responseRecorder.status == 0 {
		// The status will be StatusSwitchingProtocols if there was no error and
		// WriteHeader has not been called yet
		l.responseRecorder.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

type closeNotifyWriter struct {
	RecordingResponseWriter
	http.CloseNotifier
}

type hijackCloseNotifier struct {
	RecordingResponseWriter
	http.Hijacker
	http.CloseNotifier
}

type recordingBody struct {
	io.ReadCloser
	size int
}

func (b *recordingBody) Read(bs []byte) (n int, err error) {
	n, err = b.ReadCloser.Read(bs)
	b.size += n
	return
}

func (b *recordingBody) Size() int {
	return b.size
}
