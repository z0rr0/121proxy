// Copyright (c) 2015, Alexander Zaytsev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// It implements a simple TCP proxy server.
// It forwards incoming TCP requests to remote servers without any data changes.
//
package proxy

import (
    "testing"
    "os"
    "fmt"
    "path/filepath"
    "net"
)

const (
    // $GOPATH/testConf
    testConf = "test.conf"
)

func runReomteServer(t *testing.T) {
    conFile := filepath.Join(os.Getenv("GOPATH"), testConf)
    cfg, err := readConfig(conFile)
    if err != nil {
        t.Errorf(err)
        return
    }
    ln, err := net.Listen("tcp", fmt.Sprintf("%v:%v", cfg.OutHost[0], cfg.OutPort))
    if err != nil {
        t.Errorf(err)
        return
    }
    for {
        c, err := ln.Accept()
        if err != nil {
            t.Errorf(err)
        }
        t.Logf("\naccept from %v\n", c.RemoteAddr())
        go func() {
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
                    _, err = c.Write(b[:n])
                    if err != nil {
                        return
                    }
                }
            }
        }()
    }
}

func TestNew(t *testing.T) {
    // create "remote" TCP server
    go runReomteServer
}
