Easily run Let's Encrypt-enabled HTTP server in Go
==================================================

[![GoDoc](https://godoc.org/github.com/andreyvit/easyhttpserver?status.svg)](https://godoc.org/github.com/andreyvit/easyhttpserver)

Production-ready HTTP/HTTPS opinionated server in a few lines of code.

Modes:

* development mode (http + dev port);
* standalone production mode (http + https + obtains certificates from Let's Encrypt);
* slave production mode (http + custom port), this runs behind your reverse proxy like nginx, Apache, Caddy or on PaaS like Heroku.

Features:

* one call to auto-detect the mode and start both HTTP and HTTPS servers;
* one line to read configuration from [Heroku-style](https://12factor.net) environment variables (12-factor), or provide a custom configuration your own way;
* optional graceful shutdown on Ctrl-C, SIGKILL, SIGHUP.


Usage
-----

Recommended for [12-factor apps](https://12factor.net), load all options from environment variables:

```go
import (
    "github.com/andreyvit/easyhttpserver"
)

func main() {
    serverOpt := easyhttpserver.Options{
        DefaultDevPort:          3001,
        GracefulShutdownTimeout: 2 * time.Second, // no long-lived requests
    }

    // loads HOST, LETSENCRYPT_ENABLED, etc from environment
    err := serverOpt.LoadEnv()
    if err != nil {
        log.Fatalf("** ERROR: invalid configuration: %v", err)
    }

    srv, err := easyhttpserver.Start(http.HandlerFunc(helloWorld), serverOpt)
    if err != nil {
        log.Fatalf("** ERROR: server startup: %v", err)
    }

    // shut down on Ctrl-C, SIGKILL, SIGHUP
    easyhttpserver.InterceptShutdownSignals(srv.Shutdown)

    log.Printf("HelloWorld server running at %s", strings.Join(srv.Endpoints(), ", "))
    err = srv.Wait()
    if err != nil {
        log.Fatalf("** ERROR: %v", err)
    }
}
```

Or specify the options manually:

```go
import (
    "github.com/andreyvit/easyhttpserver"
)

func main() {
    srv, err := easyhttpserver.Start(http.HandlerFunc(helloWorld), easyhttpserver.Options{
        DefaultDevPort:          3001,
        GracefulShutdownTimeout: 2 * time.Second, // no long-lived requests
        Host:                    "myhost.example.com",
        LetsEncrypt:             true,
        LetsEncryptEmail:        "you@example.com",
        LetsEncryptCacheDir:     "~/.local/share/easyhttpserver_example/",
    })
    if err != nil {
        log.Fatalf("** ERROR: server startup: %v", err)
    }

    // shut down on Ctrl-C, SIGKILL, SIGHUP
    easyhttpserver.InterceptShutdownSignals(srv.Shutdown)

    log.Printf("HelloWorld server running at %s", strings.Join(srv.Endpoints(), ", "))
    err = srv.Wait()
    if err != nil {
        log.Fatalf("** ERROR: %v", err)
    }
}
```


[0BSD](https://opensource.org/licenses/0BSD) License
----------------------------------------------------

Copyright 2020 [Andrey Tarantsov](mailto:andrey@tarantsov.com).

Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
