//go:build windows

package main

import (
	"os"

	log "github.com/sirupsen/logrus"
)

// installHotReloadSignal is a no-op on Windows because POSIX SIGHUP is not
// available. Operators should restart the service to rotate credentials.
//
// On Windows, inventory re-registration still happens automatically via the
// plugin-registry watcher ticker (StartWatcherWithCallback) and after any
// successful Reconcile result. SIGHUP-driven tenant reload and reconcile
// re-registration are not available on this platform.
func installHotReloadSignal(c chan os.Signal) {
	log.WithField("component", "main").Info("SIGHUP hot reload unavailable on windows; use service restart for credential rotation")
}
