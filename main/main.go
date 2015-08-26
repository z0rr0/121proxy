// Copyright (c) 2015, Alexander Zaytsev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package main is a simple TCP proxy server.
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
    Name = "121proxy"
    // Comment is a general program comment
    Comment = " It is a simple TCP proxy server.\n It forwards incoming TCP requests to remote server without any data changes."
)

var (
    // Version is a program version
    Version = "v0.0"
    // Revision is CVS GIT version
    Revision = "git:000000"
    // BuildDate is date of build
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
        errc <- p.Start()
    }()
    fmt.Printf("termination, reason: %v\n", <-errc)
}
