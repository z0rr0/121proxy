// Copyright (c) 2015, Alexander Zaytsev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// It implements a simple TCP proxy server.
// It forwards incoming TCP requests to remote servers without any data changes.
//
package proxy

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	// $GOPATH/testConf
	testConf = "test.json"
)

func echoServer(t *testing.T, cfg *Config) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("%v:%v", cfg.OutHost[0], cfg.OutPort))
	if err != nil {
		t.Log(err)
		return err
	}
	for {
		c, err := ln.Accept()
		if err != nil {
			t.Log(err)
			return err
		}
		t.Logf("\naccept from %v\n", c.RemoteAddr())
		go func() {
			defer func() {
				fmt.Printf("\nclose from %v\n", c.RemoteAddr())
				c.Close()
			}()
			io.Copy(c, c)
		}()
	}
}

func TestNew(t *testing.T) {
	_, err := readConfig("badfile")
	if err == nil {
		t.Error(err)
	}
	conFile := filepath.Join(os.Getenv("GOPATH"), testConf)
	_, err = readConfig(conFile)
	if err != nil {
		t.Error(err)
	}
}

func TestStart(t *testing.T) {
	// read test configuration, expected:
	// localhost:15000 <-> localhost:10000
	conFile := filepath.Join(os.Getenv("GOPATH"), testConf)
	cfg, err := readConfig(conFile)
	if err != nil {
		t.Error(err)
	}
	errc := make(chan error)
	go func() {
		t.Errorf("abnormal action: %v", <-errc)
	}()
	// create "remote" TCP server
	go func() {
		errc <- echoServer(t, cfg)
	}()
	// init proxy
	p, err := New(conFile, true)
	if err != nil {
		t.Errorf("proxy init error: %v", err)
		return
	}
	go func() {
		errc <- p.Start()
	}()
	// create "remote" TCP client
	proxyAddr := fmt.Sprintf("%v:%v", cfg.InHost, cfg.InPort)
	raddr, err := net.ResolveTCPAddr("tcp", proxyAddr)
	if err != nil {
		t.Errorf("tcp resolve error: %v", err)
		return
	}
	time.Sleep(50 * time.Millisecond)
	con, err := net.DialTCP("tcp", nil, raddr)
	if err != nil {
		t.Errorf("tcp connection error: %v", err)
		return
	}
	defer con.Close()
	values := []string{"a", "ab", "abc", "abcd", "abcde"}
	// start reader
	go func() {
		j, v := 0, []byte(strings.Join(values, ""))
		for {
			b := make([]byte, 8)
			n, err := con.Read(b)
			if err != nil {
				t.Logf("unexpected: %v", err)
			} else {
				t.Logf("read: %v", string(b[:n]))
				for i := 0; i < n; i++ {
					if v[j+i] != b[i] {
						t.Errorf("incorrect values: %v != %v", v[j+i], b[i])
					}
				}
				j = j + n
			}
			if j == (len(v) - 1) {
				return
			}
		}
	}()
	time.Sleep(50 * time.Millisecond)
	// write
	// b := make([]byte, 8)
	for _, v := range values {
		t.Logf("try to write: %v", v)
		_, err := con.Write([]byte(v))
		if err != nil {
			t.Errorf("write error: %v", err)
		}
	}
	time.Sleep(50 * time.Millisecond)
}
