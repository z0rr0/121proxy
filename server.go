// Copyright (c) 2015, Alexander Zaytsev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
    "flag"
    "fmt"
    "io"
    "net"
)

func handleConnection(c net.Conn) {
    defer func() {
        fmt.Printf("\nclose from %v\n", c.RemoteAddr())
        c.Close()
    }()
    b := make([]byte, 8)
    for {
        n, err := c.Read(b)
        switch {
        case err == io.EOF:
            return
        case err != nil:
            fmt.Printf("error: %v\n", err)
        default:
            // 'q'
            if (n > 0) && b[0] == 113 {
                return
            }
            fmt.Printf("%v", b[:n])
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
    fmt.Printf("listen: %v\n", conStr)
    for {
        conn, err := ln.Accept()
        if err != nil {
            panic(err)
        }
        fmt.Printf("\naccept from %v\n", conn.RemoteAddr())
        go handleConnection(conn)
    }
}
