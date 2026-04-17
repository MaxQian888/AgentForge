//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// installHotReloadSignal wires SIGHUP into the provided channel so the main
// loop can react to `kill -HUP <pid>` without restart.
func installHotReloadSignal(c chan os.Signal) {
	signal.Notify(c, syscall.SIGHUP)
}
