package app

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/woxQAQ/febuildergba-mygo/fault"
)

var errNoEmulator = fault.Code{ID: "app.no_emulator", Kind: fault.NotFound}

func launchEmulator(path string) (err error) {
	defer func() { err = fault.Wrap(err, "app.emulator", fault.Context{Path: path}) }()
	if command := os.Getenv("FEBUILDER_EMULATOR"); command != "" {
		return startProcess(command, path)
	}
	if command, err := exec.LookPath("mgba"); err == nil {
		return startProcess(command, path)
	}
	if runtime.GOOS == "darwin" {
		if _, err := os.Stat("/Applications/mGBA.app"); err == nil {
			// #nosec G204 -- The ROM is a separate argument to the fixed system
			// launcher.
			return exec.Command("open", "-a", "/Applications/mGBA.app", path).
				Run()
		}
	}
	return errNoEmulator
}

func startProcess(executable string, args ...string) error {
	// #nosec G204 G702 -- The executable is explicitly configured by the user
	// or found on PATH; no shell is used.
	command := exec.Command(executable, args...)
	if err := command.Start(); err != nil {
		return fault.Wrap(
			err,
			"app.start_process",
			fault.Context{Path: executable},
		)
	}
	go func() { _ = command.Wait() }()
	return nil
}
