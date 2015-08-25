// Copyright (c) 2015, Alexander Zaytsev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net"
    "os"
    "path/filepath"
)

type Config struct {
    InPort  uint   `json:"inport"`
    InHost  string `json:"inhost"`
    OutPort uint   `json:"outport"`
    OutHost string `json:"outhost"`
    Workers uint   `json:"workers"`
}

type Proxy struct {
    cfg      *Config
    LogDebug *log.Logger
    LogError *log.Logger
    Counter  uint
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
    case cfg.Workers == 0:
        fmt.Println("WARNING: workers is not configured, unlimited mode")
    }
    if err != nil {
        return nil, err
    }
    p := &Proxy{
        cfg,
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
func (p *Proxy) Dial() (*net.TCPConn, error) {
    raddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%v:%v", p.cfg.OutHost, p.cfg.OutPort))
    if err != nil {
        p.LogError.Printf("can't resolve address: %v:%v\n", p.cfg.OutHost, p.cfg.OutPort)
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
    laddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%v:%v", p.cfg.InHost, p.cfg.InPort))
    if err != nil {
        p.LogError.Printf("can't resolve address: %v:%v\n", p.cfg.InHost, p.cfg.InPort)
        return nil, err
    }
    ln, err := net.ListenTCP("tcp", laddr)
    if err != nil {
        p.LogError.Printf("can't listen tcp: %v:%v\n", laddr.IP, laddr.Port)
        return nil, err
    }
    p.LogDebug.Printf("listen: %v:%v\n", laddr.IP, laddr.Port)
    return ln, nil
}

// Handle handles new incomming connection
func (p *Proxy) Handle(inCon *net.TCPConn) {
    const buffer = 32
    defer func() {
        p.Counter--
        inCon.Close()
    }()
    outCon, err := p.Dial()
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
    return p.Counter > p.cfg.Workers
}
