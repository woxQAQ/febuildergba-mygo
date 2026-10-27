package app

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
)

func launchEmulator(path string) error {
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
	return errors.New("no_emulator")
}

func startProcess(executable string, args ...string) error {
	// #nosec G204 G702 -- The executable is explicitly configured by the user
	// or found on PATH; no shell is used.
	command := exec.Command(executable, args...)
	if err := command.Start(); err != nil {
		return err
	}
	go func() { _ = command.Wait() }()
	return nil
}
