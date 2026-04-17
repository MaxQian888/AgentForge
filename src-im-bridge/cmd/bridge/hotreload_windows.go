//go:build windows

package main

import (
	"os"

	log "github.com/sirupsen/logrus"
)

// installHotReloadSignal is a no-op on Windows because POSIX SIGHUP is not
// available. Operators should restart the service to rotate credentials.
func installHotReloadSignal(c chan os.Signal) {
	log.WithField("component", "main").Info("SIGHUP hot reload unavailable on windows; use service restart for credential rotation")
}
