package easyhttpserver_test

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/andreyvit/easyhttpserver"
)

func Example() {
	// normally you'd set up the environment externally, and create the dir manually
	os.Setenv("LETSENCRYPT_ENABLED", "1")
	os.Setenv("LETSENCRYPT_EMAIL", "you@example.com")                            // set your email here
	os.Setenv("LETSENCRYPT_CACHE_DIR", "~/.local/share/easyhttpserver_example/") // set your dir (mod 0700) here
	os.Setenv("HOST", "myhost.example.com")                                      // set HTTPS hostname to respond to
	must(os.MkdirAll(os.Getenv("LETSENCRYPT_CACHE_DIR"), 0700))

	serverOpt := easyhttpserver.Options{
		DefaultDevPort:          3099,
		GracefulShutdownTimeout: 2 * time.Second, // no long-lived requests
	}
	must(serverOpt.LoadEnv()) // loads HOST, LETSENCRYPT_ENABLED, etc from environment

	srv, err := easyhttpserver.Start(http.HandlerFunc(helloWorld), serverOpt)
	must(err)

	easyhttpserver.InterceptShutdownSignals(srv.Shutdown) // shut down on Ctrl-C, SIGKILL, SIGHUP
	after500ms(srv.Shutdown)                              // end test after 500 ms, real servers don't do this

	fmt.Printf("HelloWorld server running at %s\n", strings.Join(srv.Endpoints(), ", "))
	must(srv.Wait())

	// Output: HelloWorld server running at https://myhost.example.com
}

func Example_manualConfig() {
	srv, err := easyhttpserver.Start(http.HandlerFunc(helloWorld), easyhttpserver.Options{
		DefaultDevPort:          3099,
		GracefulShutdownTimeout: 2 * time.Second, // no long-lived requests
		Host:                    "myhost.example.com",
		LetsEncrypt:             true,
		LetsEncryptEmail:        "you@example.com",
		LetsEncryptCacheDir:     "~/.local/share/easyhttpserver_example/",
	})
	must(err)

	easyhttpserver.InterceptShutdownSignals(srv.Shutdown) // shut down on Ctrl-C, SIGKILL, SIGHUP
	after500ms(srv.Shutdown)                              // end test after 500 ms, real servers don't do this

	fmt.Printf("HelloWorld server running at %s\n", strings.Join(srv.Endpoints(), ", "))
	must(srv.Wait())

	// Output: HelloWorld server running at https://myhost.example.com
}

func Example_defaultDevServer() {
	srv, err := easyhttpserver.Start(http.HandlerFunc(helloWorld), easyhttpserver.Options{
		DefaultDevPort:          3099,
		GracefulShutdownTimeout: 2 * time.Second, // no long-lived requests
	})
	must(err)

	easyhttpserver.InterceptShutdownSignals(srv.Shutdown) // shut down on Ctrl-C, SIGKILL, SIGHUP
	after500ms(srv.Shutdown)                              // end test after 500 ms, real servers don't do this

	fmt.Printf("HelloWorld server running at %s\n", strings.Join(srv.Endpoints(), ", "))
	must(srv.Wait())

	// Output: HelloWorld server running at http://localhost:3099
}

func helloWorld(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World!")
}

func after500ms(f func()) {
	go func() {
		time.Sleep(500 * time.Millisecond)
		f()
	}()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
