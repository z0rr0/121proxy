// Copyright (c) 2020, Alexander Zaytsev <me@axv.email>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package proxy implements a simple TCP proxy server.
// It forwards incoming TCP requests to remote servers without any data changes.
package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrClosed is shutdown error.
	ErrClosed = errors.New("closing")
	// internal logger
	info             = log.New(os.Stdout, fmt.Sprintf("INFO [121proxy]: "), log.Ldate|log.Ltime|log.Lshortfile)
	shutdownInterval = 250 * time.Millisecond
)

// NetCfg is net client settings.
type NetCfg struct {
	Host string `json:"host"`
	Port uint   `json:"port"`
}

// HostCfg is a proxy settings.
type HostCfg struct {
	Src     NetCfg        `json:"src"`
	Dst     NetCfg        `json:"dst"`
	Limit   int64         `json:"limit"`
	Buffer  int           `json:"buffer"`
	done    chan struct{} // shutdown
	counter int64
}

// Proxy is struct to store hosts configuration parameters
type Proxy struct {
	Hosts      []*HostCfg `json:"hosts"`
	Monitoring int        `json:"monitoring"`
	inShutdown int32
	listeners  []*net.TCPListener
}

// Addr returns a network address "host:port".
func (n *NetCfg) Addr() string {
	return net.JoinHostPort(n.Host, fmt.Sprint(n.Port))
}

// Name returns a host configuration settings string.
func (h *HostCfg) Name() string {
	return fmt.Sprintf("%v <-> %v", h.Src.Addr(), h.Dst.Addr())
}

// Listen starts to listen source socket.
func (h *HostCfg) Listen() (*net.TCPListener, error) {
	netAddress := h.Src.Addr()
	addr, err := net.ResolveTCPAddr("tcp", netAddress)
	if err != nil {
		return nil, fmt.Errorf("failed resolve %v: %v", netAddress, err)
	}
	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed listen %v: %v", netAddress, err)
	}
	return ln, nil
}

// Dial sets outcome connection to remote dst host.
func (h *HostCfg) Dial() (*net.TCPConn, error) {
	addr := h.Dst.Addr()
	remoteAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("can't resolve address %v: %v", addr, err)
	}
	con, err := net.DialTCP("tcp", nil, remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("can't setup tcp connection to %v: %v", addr, err)
	}
	return con, nil
}

// Run does transmission data between clients.
func (h *HostCfg) Run(inCon *net.TCPConn) error {
	defer func() {
		if err := inCon.Close(); err != nil {
			info.Printf("inCon close error: %v\n", err)
		}
	}()
	outCon, err := h.Dial()
	if err != nil {
		return err
	}
	defer func() {
		if err := outCon.Close(); err != nil {
			info.Printf("outCon close error: %v\n", err)
		}
	}()
	fromServer, fromClient := make([]byte, h.Buffer), make([]byte, h.Buffer)
	end := make(chan struct{})
	// client (incCon) <- proxy <- server (outCon)
	transmission := func(src, dst *net.TCPConn, b []byte, name string) {
		defer func() {
			end <- struct{}{}
		}()
		_, err := io.CopyBuffer(dst, src, b)
		if err != nil {
			info.Printf("can't copy data %s [%s]: %v\n", h.Name(), name, err)
			return
		}
	}
	// server -> client
	go transmission(outCon, inCon, fromServer, "server")
	// client -> server
	go transmission(inCon, outCon, fromClient, "client")
	// exit only if an error occurred or shutdown was called
	for {
		select {
		case <-end:
			return nil
		case <-h.done:
			return ErrClosed
		}
	}
}

// HandleHost accepts incoming request for new proxy connections.
func (p *Proxy) HandleHost(i int, done chan<- int) {
	var (
		wg           sync.WaitGroup
		reachedLimit bool
	)
	ln := p.listeners[i]
	h := p.Hosts[i]
	for {
		if p.shuttingDoneCalled() {
			// don't accept new connections if shutdown was called
			// but waits closing for all active ones using `wg sync.WaitGroup`
			break
		}
		if n := atomic.LoadInt64(&h.counter); n < h.Limit {
			reachedLimit = false
			inConn, err := ln.AcceptTCP()
			if err != nil {
				info.Printf("can not accept: %v\n", h.Src.Addr())
				continue
			}
			wg.Add(1)
			atomic.AddInt64(&h.counter, 1)
			go func() {
				defer func() {
					wg.Done()
					atomic.AddInt64(&h.counter, -1)
				}()
				if err := h.Run(inConn); err != nil && err != ErrClosed {
					info.Printf("failed handler run %v: %v", h.Name(), err)
				}
			}()
		} else {
			if !reachedLimit {
				info.Printf("limit %d is reached for settings=%d %s", h.Limit, i, h.Name())
				reachedLimit = true
			}
		}
	}
	wg.Wait()
	done <- i
}

// Start initializes and runs TCP proxy.
func (p *Proxy) Start() error {
	var err error
	if p.shuttingDoneCalled() {
		return ErrClosed
	}
	n := len(p.Hosts)
	p.listeners = make([]*net.TCPListener, n)
	for i, h := range p.Hosts {
		ln, err := h.Listen()
		if err != nil {
			return err
		}
		p.listeners[i] = ln
		// for shutdown, will be closed after closeListeners
		p.Hosts[i].done = make(chan struct{})
		info.Printf("listen %v\n", h.Src.Addr())
	}
	done := make(chan int, n)
	for i := range p.listeners {
		go p.HandleHost(i, done)
	}
	// periodic show hosts counters
	if p.Monitoring > 0 {
		m := time.NewTicker(time.Duration(p.Monitoring) * time.Second)
		defer m.Stop()
		go p.monitoring(m)
	}
	// wait shutdown closing from all listeners
	for range done {
		n--
		if n == 0 {
			break
		}
	}
	close(done) // all HandleHost calls were finished
	atomic.StoreInt32(&p.inShutdown, 2)
	return err
}

func (p *Proxy) monitoring(m *time.Ticker) {
	for range m.C {
		for i, h := range p.Hosts {
			info.Printf("monitoring host [%d] %s used counter %d", i, h.Name(), atomic.LoadInt64(&h.counter))
		}
	}
}

func (p *Proxy) shuttingDoneCalled() bool {
	return atomic.LoadInt32(&p.inShutdown) != 0
}

func (p *Proxy) shuttingDownFinished() bool {
	return atomic.LoadInt32(&p.inShutdown) == 2
}

// Shutdown gracefully shutdowns handlers.
func (p *Proxy) Shutdown(ctx context.Context) error {
	var err error
	atomic.StoreInt32(&p.inShutdown, 1)
	for i := range p.Hosts {
		close(p.Hosts[i].done)
		if e := p.listeners[i].Close(); e != nil && err == nil {
			err = e
		}
	}
	ticker := time.NewTicker(shutdownInterval)
	defer ticker.Stop()
	for {
		if p.shuttingDownFinished() {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			//return errors.New("timed out")
		}
	}
}

// New creates new proxy structure.
func New(fileName string) (*Proxy, error) {
	p := &Proxy{}
	fullPath, err := filepath.Abs(fileName)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, p)
	if err != nil {
		return nil, err
	}
	if n := len(p.Hosts); n < 1 {
		return nil, fmt.Errorf("no cofiguration hosts")
	}
	for i := range p.Hosts {
		if p.Hosts[i].Limit < 1 {
			return nil, fmt.Errorf("failed limit for host #%v", i)
		}
		if p.Hosts[i].Buffer < 1 {
			return nil, fmt.Errorf("failed buffer value for host #%v", i)
		}
	}
	return p, nil
}
