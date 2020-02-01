// Copyright (c) 2020, Alexander Zaytsev <me@axv.email>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package main is a simple echo server.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

var (
	// internal logger
	info = log.New(os.Stdout, fmt.Sprintf("INFO [server]: "), log.Ldate|log.Ltime)
)

func handleConnection(c net.Conn) {
	defer func() {
		info.Printf("close connection from %v\n", c.RemoteAddr())
		if err := c.Close(); err != nil {
			info.Printf("failed conneciton close: %v\n", err)
		}
	}()
	b := make([]byte, 8)
	for {
		n, err := c.Read(b)
		switch {
		case err == io.EOF:
			return
		case err != nil:
			info.Printf("error: %v\n", err)
		default:
			// 'q'
			if (n > 0) && b[0] == 113 {
				return
			}
			info.Printf("%v", b[:n])
			_, err = c.Write(b[:n])
			if err != nil {
				return
			}
		}
	}
}

func main() {
	port := flag.Uint("port", 10000, "listen port")
	flag.Parse()

	conStr := fmt.Sprintf(":%v", *port)
	ln, err := net.Listen("tcp", conStr)
	if err != nil {
		panic(err)
	}
	info.Printf("listen: %v\n", conStr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		info.Printf("accept connection from %v\n", conn.RemoteAddr())
		go handleConnection(conn)
	}
}
