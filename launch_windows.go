//go:build windows

package main

import (
	"errors"
	"os"
	"os/exec"
)

// launchAgy roda o agy ligado ao terminal e sai com o mesmo código dele
// (o Windows não tem exec que substitui o processo).
func launchAgy(bin string, args []string) error {
	c := exec.Command(bin, args...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := c.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		os.Exit(ee.ExitCode())
	}
	if err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
