package easyhttpserver

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

// InterceptShutdownSignals invokes the given function the first time INT
// (Ctrl-C), KILL or HUP signal is received. This only happens once;
// the next signal received will kill the app.
func InterceptShutdownSignals(shutdown func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGHUP)
	go func() {
		<-c
		signal.Reset()
		log.Println("shutting down, interrupt again to force quit")
		shutdown()
	}()
}
