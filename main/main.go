// Copyright (c) 2015, Alexander Zaytsev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
    "flag"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "github.com/z0rr0/121proxy/proxy"
)

const (
    // Config is a default configuration file name
    Config = "config.conf"
    // Name is a program name
    Name    = "121proxy"
    Comment = " It is a simple TCP proxy server.\n It forwards incoming TCP requests to remote server without any data changes."
)

var (
    Version   = "v0.0"
    Revision  = "git:000000"
    BuildDate = "1900-00-00_00:00:00UTC"
)

func interrupt() error {
    c := make(chan os.Signal)
    signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
    return fmt.Errorf("%v", <-c)
}

func main() {
    defer func() {
        if r := recover(); r != nil {
            fmt.Printf("abnormal termination, reason: %v\n", r)
        }
    }()
    errc := make(chan error)
    go func() {
        errc <- interrupt()
    }()
    debug := flag.Bool("debug", false, "debug mode")
    version := flag.Bool("version", false, "print version info")
    config := flag.String("config", Config, "configuration file")
    flag.Parse()
    if *version {
        fmt.Printf("%v: %v %v %v\n%v\n", Name, Version, Revision, BuildDate, Comment)
        // flag.PrintDefaults()
        return
    }
    p, err := proxy.New(*config, *debug)
    if err != nil {
        panic(err)
    }
    go func() {
        ln, err := p.Listen()
        if err != nil {
            errc <- err
            return
        }
        for {
            inConn, err := ln.AcceptTCP()
            if err != nil {
                p.LogError.Printf("can't accept incoming connection: %v\n", err)
                continue
            }
            p.Counter++
            if p.LimitReached() {
                p.LogError.Printf("can't accept incoming connection from %v: workers' limit is reached (%v)\n", inConn.RemoteAddr(), p.Counter)
                inConn.Close()
                continue
            }
            go p.Handle(inConn)
        }
    }()
    fmt.Printf("termination, reason: %v\n", <-errc)
}
