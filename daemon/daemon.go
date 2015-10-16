// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2015 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package daemon

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-systemd/activation"
	"github.com/gorilla/mux"
	"gopkg.in/tomb.v2"

	"launchpad.net/snappy/logger"
)

// A Daemon listens for requests and routes them to the right command
type Daemon struct {
	sync.RWMutex // for concurrent access to the tasks map
	tasks        map[string]*Task
	listener     net.Listener
	tomb         tomb.Tomb
	router       *mux.Router
}

// A ResponseFunc handles one of the individual verbs for a method
type ResponseFunc func(*Command, *http.Request) Response

// A Command routes a request to an individual per-verb ResponseFUnc
type Command struct {
	Path string
	//
	GET    ResponseFunc
	PUT    ResponseFunc
	POST   ResponseFunc
	DELETE ResponseFunc
	//
	d *Daemon
}

func (c *Command) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var rspf ResponseFunc
	var rsp Response = BadMethod

	switch r.Method {
	case "GET":
		rspf = c.GET
	case "PUT":
		rspf = c.PUT
	case "POST":
		rspf = c.POST
	case "DELETE":
		rspf = c.DELETE
	}
	if rspf != nil {
		rsp = rspf(c, r)
	}

	rsp.ServeHTTP(w, r)
}

type wrappedWriter struct {
	w http.ResponseWriter
	s int
}

func (w *wrappedWriter) Header() http.Header {
	return w.w.Header()
}

func (w *wrappedWriter) Write(bs []byte) (int, error) {
	return w.w.Write(bs)
}

func (w *wrappedWriter) WriteHeader(s int) {
	w.w.WriteHeader(s)
	w.s = s
}

func logit(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := &wrappedWriter{w: w}
		t0 := time.Now()
		handler.ServeHTTP(ww, r)
		t := time.Now().Sub(t0)
		logger.Debugf("%s %s %s %s %d", r.RemoteAddr, r.Method, r.URL, t, ww.s)
	})
}

// Init sets up the Daemon's internal workings.
// Don't call more than once.
func (d *Daemon) Init() error {
	t0 := time.Now()
	listeners, err := activation.Listeners(false)
	if err != nil {
		return err
	}

	if len(listeners) != 1 {
		return fmt.Errorf("daemon does not handle %d listeners right now, just one", len(listeners))
	}

	d.listener = listeners[0]

	d.addRoutes()

	logger.Debugf("init done in %s", time.Now().Sub(t0))

	return nil
}

func (d *Daemon) addRoutes() {
	d.router = mux.NewRouter()

	for _, c := range api {
		c.d = d
		logger.Debugf("adding %s", c.Path)
		d.router.Handle(c.Path, c).Name(c.Path)
	}

	// also maybe add a /favicon.ico handler...

	d.router.NotFoundHandler = NotFound
}

// Start the Daemon
func (d *Daemon) Start() {
	d.tomb.Go(func() error {
		return http.Serve(d.listener, logit(d.router))
	})
}

// Stop shuts down the Daemon
func (d *Daemon) Stop() error {
	d.tomb.Kill(nil)
	return d.tomb.Wait()
}

// Dying is a tomb-ish thing
func (d *Daemon) Dying() <-chan struct{} {
	return d.tomb.Dying()
}

// AddTask runs the given function as a task
func (d *Daemon) AddTask(f func() interface{}) *Task {
	t := RunTask(f)
	d.Lock()
	defer d.Unlock()
	d.tasks[t.UUID()] = t

	return t
}

// GetTask retrieves a task from the tasks map, by uuid.
func (d *Daemon) GetTask(uuid string) *Task {
	d.RLock()
	defer d.RUnlock()
	return d.tasks[uuid]
}

var (
	errTaskNotFound     = errors.New("task not found")
	errTaskStillRunning = errors.New("task still running")
)

// DeleteTask removes a task from the tasks map, by uuid.
func (d *Daemon) DeleteTask(uuid string) error {
	d.Lock()
	defer d.Unlock()
	task, ok := d.tasks[uuid]
	if !ok || task == nil {
		return errTaskNotFound
	}
	if task.State() != TaskRunning {
		delete(d.tasks, uuid)
		return nil
	}

	return errTaskStillRunning
}

// New Daemon
func New() *Daemon {
	return &Daemon{
		tasks: make(map[string]*Task),
	}
}
