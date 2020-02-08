# 121proxy

![Go](https://github.com/z0rr0/121proxy/workflows/Go/badge.svg)
[![GoDoc](https://godoc.org/github.com/z0rr0/121proxy/proxy?status.svg)](https://godoc.org/github.com/z0rr0/121proxy/proxy)

One to one TCP proxy.

It is a simple TCP proxy server. It forwards data from remote its client to another server without any modifications. Each connection client->proxy is mapped to according one proxy->server

```
client <-> proxy <-> server
```

---

*This source code is governed by a [BSD-3-Clause](http://opensource.org/licenses/BSD-3-Clause) license that can be found in the [LICENSE](https://github.com/z0rr0/121proxy/blob/master/LICENSE) file.
