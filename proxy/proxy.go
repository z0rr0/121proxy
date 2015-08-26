// Copyright (c) 2015, Alexander Zaytsev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// It implements a simple TCP proxy server.
// It forwards incoming TCP requests to remote servers without any data changes.
//
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
)

// Config is struct to store configuration parameters
type Config struct {
    InPort  uint     `json:"inport"`
    InHost  string   `json:"inhost"`
    OutPort uint     `json:"outport"`
    OutHost []string `json:"outhost"`
    Workers uint     `json:"workers"`
}

// Proxy is main struct to control proxy.
type Proxy struct {
    cfg      *Config
    servers  int
    mutex    sync.Mutex
    LogDebug *log.Logger
    LogError *log.Logger
    Counter  uint
}

// getServer returns a current remote network address.
// It can be used for easy load balancing.
func (p *Proxy) getServer() string {
    p.mutex.Lock()
    lhosts := len(p.cfg.OutHost)
    defer func() {
        p.servers++
        if p.servers >= lhosts {
            p.servers = 0
        }
        p.mutex.Unlock()
    }()
    return fmt.Sprintf("%v:%v", p.cfg.OutHost[p.servers%lhosts], p.cfg.OutPort)
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

// New creates new proxy structure
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
    case cfg.Workers == 0:
        fmt.Println("WARNING: workers is not configured, unlimited mode")
    }
    if err != nil {
        return nil, err
    }
    p := &Proxy{
        cfg, 0, sync.Mutex{},
        log.New(ioutil.Discard, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile),
        log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
        0,
    }
    if debug {
        p.LogDebug = log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
    }
    return p, nil
}

// Dial sets outcome connection
func (p *Proxy) Dial(addr string) (*net.TCPConn, error) {
    raddr, err := net.ResolveTCPAddr("tcp", addr)
    if err != nil {
        p.LogError.Printf("can't resolve address: %v\n", addr)
        return nil, err
    }
    con, err := net.DialTCP("tcp", nil, raddr)
    if err != nil {
        p.LogError.Printf("can't set tcp connection to %v:%v\n", raddr.IP, raddr.Port)
        return nil, err
    }
    return con, nil
}

// Listen start to listen a socket
func (p *Proxy) Listen() (*net.TCPListener, error) {
    // set incoming connection
    addr := fmt.Sprintf("%v:%v", p.cfg.InHost, p.cfg.InPort)
    laddr, err := net.ResolveTCPAddr("tcp", addr)
    if err != nil {
        p.LogError.Printf("can't resolve address: %v\n", addr)
        return nil, err
    }
    ln, err := net.ListenTCP("tcp", laddr)
    if err != nil {
        p.LogError.Printf("can't listen tcp: %v:%v\n", laddr.IP, laddr.Port)
        return nil, err
    }
    // ok, print info
    fmt.Printf("Listen: %v\n", addr)
    fmt.Printf("Remote servers: %v:%v\n", strings.Join(p.cfg.OutHost, ","), p.cfg.OutPort)
    return ln, nil
}

// Handle handles new incomming connection
func (p *Proxy) Handle(inCon *net.TCPConn) {
    const buffer = 32
    defer func() {
        inCon.Close()
        p.Counter--
    }()
    outCon, err := p.Dial(p.getServer())
    if err != nil {
        p.LogError.Printf("handle connection error: %v\n", err)
        return
    }
    defer outCon.Close()
    end := make(chan bool)
    fromServer, fromClient := make([]byte, buffer), make([]byte, buffer)
    p.LogDebug.Printf("session: %v-%v <-> %v-%v\n",
        inCon.LocalAddr(), inCon.RemoteAddr(),
        outCon.LocalAddr(), outCon.RemoteAddr())
    // client (incCon) <- proxy <- server (outCon)
    go func() {
        defer func() {
            end <- false
        }()
        for {
            n, err := outCon.Read(fromServer)
            if err != nil {
                p.LogError.Printf("can't read server's data: %v", err)
                return
            }
            n, err = inCon.Write(fromServer[:n])
            if err != nil {
                p.LogError.Printf("can't write to client's socket: %v", err)
                return
            }
        }
    }()
    // client (incCon) -> proxy -> server (outCon)
    go func() {
        defer func() {
            end <- true
        }()
        for {
            n, err := inCon.Read(fromClient)
            if err != nil {
                p.LogError.Printf("can't read client's data: %v", err)
                return
            }
            n, err = outCon.Write(fromClient[:n])
            if err != nil {
                p.LogError.Printf("can't write to server's socket: %v", err)
                return
            }
        }
    }()
    p.LogDebug.Printf("finish session [c-s direction=%v]\n", <-end)
}

// LimitReached validates workers' limit
func (p *Proxy) LimitReached() bool {
    if p.cfg.Workers != 0 {
        return false
    }
    return p.Counter > p.cfg.Workers
}
