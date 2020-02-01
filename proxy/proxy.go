// Copyright (c) 2020, Alexander Zaytsev <me@axv.email>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package proxy implements a simple TCP proxy server.
// It forwards incoming TCP requests to remote servers without any data changes.
package proxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// Config is struct to store configuration parameters
type Config struct {
	InPort  uint     `json:"inport"`
	InHost  string   `json:"inhost"`
	OutPort uint     `json:"outport"`
	OutHost []string `json:"outhost"`
	Workers [2]uint  `json:"workers"`
}

// Proxy is main struct to control proxy.
type Proxy struct {
	cfg      *Config
	servers  int
	mutex    sync.Mutex
	LogDebug *log.Logger
	LogInfo  *log.Logger
	counter  int64
	inAddr   string
	outAddr  []string
}

// conn is proxy connection data
type conn struct {
	num   int64
	proxy *Proxy
	c     *net.TCPConn
}

// getServer returns a current remote network address
// using round robin. It can be used for easy load balancing.
func (p *Proxy) getServer() string {
	p.mutex.Lock()
	lhosts := len(p.outAddr)
	defer func() {
		p.servers++
		if p.servers >= lhosts {
			p.servers = 0
		}
		p.mutex.Unlock()
	}()
	return p.outAddr[p.servers%lhosts]
}

// readConfig read a configuratin file
func readConfig(name string) (*Config, error) {
	cfg := &Config{}
	fullpath, err := filepath.Abs(name)
	if err != nil {
		return cfg, err
	}
	_, err = os.Stat(fullpath)
	if err != nil {
		return cfg, err
	}
	jsondata, jerr := ioutil.ReadFile(fullpath)
	if jerr != nil {
		return cfg, err
	}
	err = json.Unmarshal(jsondata, cfg)
	return cfg, err
}

// New creates new proxy structure.
func New(filename string, debug bool) (*Proxy, error) {
	const ermsg string = "incorrect configuration parameter: %v"
	cfg, err := readConfig(filename)
	if err != nil {
		return nil, err
	}
	switch {
	case cfg.InPort == 0:
		err = fmt.Errorf(ermsg, "inport")
	case cfg.OutPort == 0:
		err = fmt.Errorf(ermsg, "outport")
	case len(cfg.OutHost) == 0:
		err = fmt.Errorf(ermsg, "outhost")
	case (cfg.Workers[0] == 0) || (cfg.Workers[1] == 0):
		fmt.Println("WARNING: workers is not configured, unlimited mode")
	}
	if err != nil {
		return nil, err
	}
	outAddr := make([]string, len(cfg.OutHost))
	for i, host := range cfg.OutHost {
		outAddr[i] = net.JoinHostPort(host, fmt.Sprint(cfg.OutPort))
	}
	p := &Proxy{
		cfg:      cfg,
		LogDebug: log.New(ioutil.Discard, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile),
		LogInfo:  log.New(os.Stderr, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		inAddr:   net.JoinHostPort(cfg.InHost, fmt.Sprint(cfg.InPort)),
		outAddr:  outAddr,
	}
	if debug {
		p.LogDebug = log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
	}
	return p, nil
}

// Dial sets outcome connection
func (p *Proxy) Dial() (*net.TCPConn, error) {
	addr := p.getServer()
	raddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		p.LogInfo.Printf("can't resolve address: %v\n", addr)
		return nil, err
	}
	con, err := net.DialTCP("tcp", nil, raddr)
	if err != nil {
		p.LogInfo.Printf("can't setup tcp connection to %v:%v\n", raddr.IP, raddr.Port)
		return nil, err
	}
	return con, nil
}

// Listen start to listen a socket
func (p *Proxy) Listen() (*net.TCPListener, error) {
	// set incoming connection
	laddr, err := net.ResolveTCPAddr("tcp", p.inAddr)
	if err != nil {
		p.LogInfo.Printf("can't resolve address: %v\n", p.inAddr)
		return nil, err
	}
	ln, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		p.LogInfo.Printf("can't listen tcp: %v:%v\n", laddr.IP, laddr.Port)
		return nil, err
	}
	// ok, print info
	fmt.Printf("Listen: %v\n", p.inAddr)
	fmt.Printf("Remote servers: %v\n", strings.Join(p.outAddr, ", "))
	return ln, nil
}

// Handle handles new incomming connection
func (p *Proxy) Handle(inCon *net.TCPConn, num int64) {
	const buffer = 32
	defer func() {
		inCon.Close()
		atomic.AddInt64(&p.counter, -1)
	}()
	outCon, err := p.Dial()
	if err != nil {
		p.LogInfo.Printf("handle connection error: %v\n", err)
		return
	}
	defer outCon.Close()
	end := make(chan string)
	fromServer, fromClient := make([]byte, buffer), make([]byte, buffer)
	p.LogDebug.Printf("session [%v]: %v-%v <-> %v-%v\n", num,
		inCon.LocalAddr(), inCon.RemoteAddr(),
		outCon.LocalAddr(), outCon.RemoteAddr())
	// client (incCon) <- proxy <- server (outCon)
	transmission := func(in, out *net.TCPConn, b []byte, name string) {
		defer func() {
			end <- name
		}()
		for {
			n, err := in.Read(b)
			if err != nil {
				p.LogDebug.Printf("can't read data [%s]: %v", name, err)
				return
			}
			n, err = out.Write(b[:n])
			if err != nil {
				p.LogDebug.Printf("can't write data [%s]: %v", name, err)
				return
			}
		}
	}
	// server -> client
	go transmission(outCon, inCon, fromServer, "server")
	// client -> server
	go transmission(inCon, outCon, fromClient, "client")
	p.LogDebug.Printf("finish session[%v] [initiator=%v]\n", num, <-end)
}

// worker get incoming request from channel ch and
// runs its handler.
func worker(ch chan conn) {
	for cn := range ch {
		cn.proxy.Handle(cn.c, cn.num)
	}
}

// Start stats TCP proxy
func (p *Proxy) Start() error {
	var i uint
	ln, err := p.Listen()
	if err != nil {
		return err
	}
	ch := make(chan conn, p.cfg.Workers[1])
	for i = 0; i < p.cfg.Workers[0]; i++ {
		go worker(ch)
	}
	for {
		// handler should close this connection late
		inConn, err := ln.AcceptTCP()
		if err != nil {
			p.LogInfo.Printf("can't accept incoming connection: %v\n", err)
			continue
		}
		num := atomic.AddInt64(&p.counter, 1)
		ch <- conn{num, p, inConn}
	}
}
