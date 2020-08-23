package easyhttpserver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// Options provide the necessary settings for starting a server and talking to
// Let's Encrypt.
type Options struct {
	// App settings

	// Log is a log.Printf-style function to output logs
	Log func(format string, v ...interface{})
	// DefaultDevPort is the HTTP port to use when running on localhost.
	DefaultDevPort int
	// GracefulShutdownTimeout is the time to allow existing requests to complete
	// when trying to shut down gracefully. After this delay, all existing
	// connections are killed.
	GracefulShutdownTimeout time.Duration

	// Env settings

	// Port sets the HTTP port to use. Let's Encrypt mode requires port 80.
	Port int
	// Host sets the domain name to provide HTTPS certificates for.
	Host string
	// LetsEncrypt is the master toggle that enables Let's Encrypt.
	LetsEncrypt bool
	// LetsEncryptCacheDir is the directory to store Let's Encrypt certificates
	// and keys in. This directory should be secured as much as possible.
	LetsEncryptCacheDir string
	// LetsEncryptEmail is the email address to use for Let's Encrypt certificates.
	// Let's Encrypt might send important notifications to this email.
	LetsEncryptEmail string

	// Derived settings

	// IsLocalDevelopmentHost signals that Host is a localhost address. In this
	// mode, Port defaults to DefaultDevPort, and scheme is http. Incompatible
	// with LetsEncrypt mode.
	IsLocalDevelopmentHost bool
	// PrimaryScheme is either https or http, depending on whether Let's Encrypt
	// is enabled.
	PrimaryScheme string
}

// Server represents both the HTTP and the HTTPS servers started via this package.
type Server struct {
	httpServer  *http.Server
	httpsServer *http.Server
	errc        <-chan error

	gracefulShutdownTimeout time.Duration

	log       func(format string, v ...interface{})
	baseURL   string
	endpoints []string
}

// LoadEnv reads configuration options from the environment variables.
func (sopt *Options) LoadEnv() error {
	if s := os.Getenv("PORT"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("invalid value of PORT %q: %w", s, err)
		}
		sopt.Port = v
	}

	if s := os.Getenv("HOST"); s != "" {
		if strings.Contains(s, "//") || strings.HasPrefix(s, "http:") || strings.HasPrefix(s, "https:") {
			return fmt.Errorf("invalid value of HOST %q: schema is not allowed", s)
		}
		sopt.Host = s
	}

	if s := os.Getenv("LETSENCRYPT_EMAIL"); s != "" {
		if !strings.Contains(s, "@") {
			return fmt.Errorf("invalid value of LETSENCRYPT_EMAIL %q: missing @", s)
		}
		sopt.LetsEncryptEmail = s
	}

	if s := os.Getenv("LETSENCRYPT_CACHE_DIR"); s != "" {
		sopt.LetsEncryptCacheDir = s
	}

	if s := os.Getenv("LETSENCRYPT_ENABLED"); s != "" {
		v, ok := parseBool(s)
		if !ok {
			return fmt.Errorf("invalid boolean value of LETSENCRYPT_ENABLED %q", s)
		}
		sopt.LetsEncrypt = v
	}

	return nil
}

// Verify makes sure the options are set correctly, sets up default values for
// Host, Port and IsLocalDevelopmentHost, and sets the PrimaryScheme.
func (sopt *Options) Verify() error {
	if sopt.Host == "" || sopt.Host == "localhost" || strings.HasPrefix(sopt.Host, "localhost:") {
		sopt.IsLocalDevelopmentHost = true
	}

	if sopt.Port == 0 {
		if sopt.IsLocalDevelopmentHost {
			if sopt.DefaultDevPort == 0 {
				return fmt.Errorf("missing PORT for local development")
			}
			sopt.Port = sopt.DefaultDevPort
		} else {
			sopt.Port = 80
		}
	}

	if sopt.LetsEncrypt {
		if sopt.IsLocalDevelopmentHost {
			return fmt.Errorf("Let's Encrypt is not supported on localhost")
		}
		if sopt.Host == "" {
			return fmt.Errorf("missing HOST when LETSENCRYPT_ENABLED is true")
		}
		if sopt.LetsEncryptEmail == "" {
			return fmt.Errorf("missing LETSENCRYPT_EMAIL when LETSENCRYPT_ENABLED is true")
		}
		if sopt.LetsEncryptCacheDir == "" {
			return fmt.Errorf("missing LETSENCRYPT_CACHE_DIR when LETSENCRYPT_ENABLED is true")
		}
		if sopt.Port != 80 {
			return fmt.Errorf("Let's Encrypt requires HTTP port to be 80, got port %d instead", sopt.Port)
		}
	}

	if sopt.IsLocalDevelopmentHost {
		sopt.PrimaryScheme = "http"
	} else {
		sopt.PrimaryScheme = "https"
	}

	if sopt.Host == "" || sopt.Host == "localhost" {
		sopt.Host = "localhost:" + strconv.Itoa(sopt.Port)
	}

	return nil
}

// Returns the preferred scheme and host to contact this server. This can be used
// for links in emails, etc.
func (sopt Options) BaseURL() string {
	if sopt.PrimaryScheme == "" {
		panic("BaseURL before Verify")
	}
	if sopt.Host == "" {
		panic("missing HOST")
	}
	return fmt.Sprintf("%s://%s", sopt.PrimaryScheme, sopt.Host)
}

// Start creates a server and starts listening. It makes sure that everything is
// set up properly, and returns after both HTTP and HTTPS (if configured) servers
// start accepting connections.
//
// After this call returns, you are supposed to log a message saying the server
// is running. Call Wait() to block until the server shuts down.
func Start(handler http.Handler, sopt Options) (*Server, error) {
	if sopt.PrimaryScheme == "" {
		if err := sopt.Verify(); err != nil {
			return nil, err
		}
	}

	if sopt.GracefulShutdownTimeout == 0 {
		sopt.GracefulShutdownTimeout = 10 * time.Second
	}

	errc := make(chan error, 2)
	srv := &Server{
		errc:                    errc,
		gracefulShutdownTimeout: sopt.GracefulShutdownTimeout,
		baseURL:                 sopt.BaseURL(),
	}

	httpHandler := handler

	if acme.LetsEncryptURL != "https://acme-v02.api.letsencrypt.org/directory" {
		return nil, fmt.Errorf("ACMEv2 is not supported by this Go build (%s): acme.LetsEncryptURL = %q", runtime.Version(), acme.LetsEncryptURL)
	}

	if sopt.LetsEncrypt {
		info, err := os.Stat(sopt.LetsEncryptCacheDir)
		if err != nil {
			return nil, fmt.Errorf("cannot access Let's Encrypt cache dir %q: %w", sopt.LetsEncryptCacheDir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("Let's Encrypt cache dir %q is not a directory", sopt.LetsEncryptCacheDir)
		}

		mgr := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      autocert.DirCache(sopt.LetsEncryptCacheDir),
			HostPolicy: autocert.HostWhitelist(sopt.Host),
			Email:      sopt.LetsEncryptEmail,
			// Client: &acme.Client{
			// 	DirectoryURL: ,
			// },
		}

		srv.httpsServer = &http.Server{
			Addr:      ":https",
			Handler:   handler,
			TLSConfig: mgr.TLSConfig(),
		}

		httpHandler = mgr.HTTPHandler(nil)

		go func() {
			err := srv.httpsServer.ListenAndServeTLS("", "")
			if err == http.ErrServerClosed {
				err = nil
			}
			errc <- err
		}()
	}

	srv.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", sopt.Port),
		Handler: httpHandler,
	}
	go func() {
		err := srv.httpServer.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		errc <- err
	}()

	srv.endpoints = append(srv.endpoints, sopt.BaseURL())
	if !sopt.IsLocalDevelopmentHost && sopt.Port != 80 {
		srv.endpoints = append(srv.endpoints, fmt.Sprintf("127.0.0.1:%d", sopt.Port))
	}

	return srv, nil
}

// Endpoints returns the list of URLs the server can be reached at, meant to be
// used for internal messaging and logging. The first item is BaseURL() and is
// the preferred endpoint; extra endpoints provide the alternatives.
//
// Note that these values are for ease of debugging, and aren't necessarily valid URLs.
// E.g. some will lack a scheme.
func (srv *Server) Endpoints() []string {
	return srv.endpoints
}

// BaseURL returns the preferred scheme and host of the server. Depending on
// the current settings, it might be something like https://your.externalhost.com/
// or something more like http://localhost:3001/.
func (srv *Server) BaseURL() string {
	return srv.baseURL
}

// Wait waits until the server shuts down (either because Shutdown was called,
// or there's an error accepting connections).
func (srv *Server) Wait() error {
	var err error
	if srv.httpServer != nil {
		e := <-srv.errc
		if err == nil {
			err = e
		}
	}
	if srv.httpsServer != nil {
		e := <-srv.errc
		if err == nil {
			err = e
		}
	}

	return err
}

// Shutdown stops accepting new connections, then waits for GracefulShutdownTimeout
// for existing requests to be finished, and then forcefully closes all connections.
func (srv *Server) Shutdown() {
	gracefulShutdown(srv.Log, srv.gracefulShutdownTimeout, func(ctx context.Context) error {
		err := srv.httpServer.Shutdown(ctx)
		if srv.httpsServer != nil {
			err2 := srv.httpsServer.Shutdown(ctx)
			if err == nil {
				err = err2
			}
		}
		return err
	}, func() {
		srv.httpServer.Close()
		if srv.httpsServer != nil {
			srv.httpsServer.Close()
		}
	})
}

// gracefulShutdown tries to do a graceful shutdown, but abandons the attempt and
// performs a forceful shutdown after a timeout.
func gracefulShutdown(log func(format string, v ...interface{}), gracePeriod time.Duration, graceful func(ctx context.Context) error, forceful func()) {
	defer forceful()

	ctx, cancel := context.WithTimeout(context.Background(), gracePeriod)
	defer cancel()

	err := graceful(ctx)
	if err == context.DeadlineExceeded {
		log("graceful shutdown timed out, will close connections forcibly")
	} else if err != nil {
		panic(err)
	}
}

func parseBool(s string) (v bool, ok bool) {
	switch strings.ToLower(s) {
	case "true", "t", "yes", "y", "on", "1":
		return true, true
	case "false", "f", "no", "n", "off", "0":
		return false, true
	default:
		return false, false
	}
}

// Log logs to the opt.Log function provided when starting the server.
func (srv *Server) Log(format string, args ...interface{}) {
	if srv.log != nil {
		srv.log(format, args...)
	}
}
