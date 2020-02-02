// Copyright (c) 2020, Alexander Zaytsev <me@axv.email>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package main is a simple TCP proxy server.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/z0rr0/121proxy/proxy"
)

const (
	// Config is a default configuration file name
	Config = "config.json"
	// Name is a program name
	Name = "121proxy"
	// Comment is a general program comment
	Comment = " It is a simple TCP proxy server.\n " +
		"It forwards incoming TCP requests to remote server without any data changes."
)

var (
	// Version is a program version
	Version = "0.0.0"
	// Revision is CVS GIT version
	Revision = "git:000000"
	// BuildDate is date of build
	BuildDate = "1900-00-00_00:00:00UTC"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("abnormal termination, reason: %v\n", r)
		}
	}()
	version := flag.Bool("version", false, "print version info")
	config := flag.String("config", Config, "configuration file")
	flag.Parse()
	if *version {
		fmt.Printf("%v %v %v %v %v\n%v\n", Name, Version, Revision, runtime.Version(), BuildDate, Comment)
		// flag.PrintDefaults()
		return
	}
	p, err := proxy.New(*config)
	if err != nil {
		panic(err)
	}
	c := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
		<-sigint

		if err := p.Shutdown(context.Background()); err != nil {
			fmt.Printf("shutdown error: %v\n", err)
		}
		close(c)
	}()
	if err := p.Start(); err != nil {
		fmt.Printf("proxy error: %v\n", err)
	}
	<-c
	fmt.Println("termination")
}
