// Copyright (c) 2020, Alexander Zaytsev <me@axv.email>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// It implements a simple TCP proxy server.
// It forwards incoming TCP requests to remote servers without any data changes.
//
package proxy

import (
	"context"
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
	testConf = "121cfg.json"
)

var (
	cfgPath = filepath.Join(os.TempDir(), testConf)
)

func echoServer(t *testing.T, port uint, ch chan net.Listener) error {
	ln, err := net.Listen("tcp", net.JoinHostPort("localhost", fmt.Sprint(port)))
	if err != nil {
		t.Error(err)
		return err
	}
	ch <- ln
	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		t.Logf("\naccept from %v\n", c.RemoteAddr())
		go func() {
			defer func() {
				if e := c.Close(); e != nil {
					t.Logf("failed close echo server: %v\n", err)
				}
			}()
			if _, e := io.Copy(c, c); e != nil {
				t.Errorf("failed copy in echo server")
			}
		}()
	}
}

func TestNew(t *testing.T) {
	_, err := New("badfile")
	if err == nil {
		t.Error(err)
	}
	_, err = New(cfgPath)
	if err != nil {
		t.Error(err)
	}
}

func TestStart(t *testing.T) {
	// read test configuration, expected:
	// localhost:15001 <-> localhost:15002
	//conFile := filepath.Join(os.Getenv("GOPATH"), testConf)
	p, err := New(cfgPath)
	if err != nil {
		t.Error(err)
	}
	errc := make(chan error)
	// create "remote" TCP server
	echoListener := make(chan net.Listener)
	serverPort := p.Hosts[0].Dst.Port
	go func() {
		if err := echoServer(t, serverPort, echoListener); err != nil {
			info.Printf("echo server stop: %v\n", err)
			//t.Logf("echo server stop: %v\n", err)
		}
	}()
	go func() {
		l := <-echoListener
		errc <- p.Start()
		if err := l.Close(); err != nil {
			t.Logf("echo server listener error: %v", err)
		}
	}()
	// create "remote" TCP client
	proxyAddr := p.Hosts[0].Src.Addr()
	raddr, err := net.ResolveTCPAddr("tcp", proxyAddr)
	if err != nil {
		t.Errorf("tcp resolve error: %v", err)
		return
	}
	time.Sleep(1000 * time.Millisecond) // wait proxy start
	con, err := net.DialTCP("tcp", nil, raddr)
	if err != nil {
		t.Errorf("tcp connection error: %v", err)
		return
	}
	writeDone := make(chan bool)
	readDone := make(chan bool)

	values := []string{"a", "ab", "abc", "abcd", "abcde"}
	// start client reader
	go func() {
		defer close(readDone)
		j, v := 0, []byte(strings.Join(values, ""))
		for {
			b := make([]byte, 1)
			n, err := con.Read(b)
			if err != nil {
				t.Logf("unexpected: %v", err)
				return
			}
			//t.Logf("read: %v", string(b[:n]))
			for i := 0; i < n; i++ {
				if v[j+i] != b[i] {
					t.Errorf("incorrect values: %v != %v", v[j+i], b[i])
				}
			}
			j = j + n
			if j == (len(v) - 1) {
				//t.Log("all data read")
				return
			}
		}
	}()
	// client writer
	go func() {
		for _, v := range values {
			//t.Logf("try to write: %v", v)
			_, err := con.Write([]byte(v))
			if err != nil {
				t.Errorf("write error: %v", err)
			}
		}
		// wait read all data before shutdown
		<-readDone
		if err := p.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown error: %v\n", err)
		}
		time.Sleep(1000 * time.Millisecond)
		close(writeDone)
	}()
	if err := <-errc; err != nil {
		t.Error(err)
	}
	<-writeDone
}
