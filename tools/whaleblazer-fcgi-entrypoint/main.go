package main

import (
	"fmt"
	_ "github.com/joho/godotenv/autoload"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"syscall"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		_, _ = fmt.Fprintln(os.Stderr, "Argument required (command to run)")
		os.Exit(1)
	}

	fcgiPath := path.Dir(os.Getenv("WHALEBLAZER_FCGI_SOCK"))
	if fcgiPath == "" || fcgiPath == "/" || fcgiPath == "." {
		_, _ = fmt.Fprintln(os.Stderr, "WHALEBLAZER_FCGI_SOCK is not set or is invalid")
		os.Exit(1)
	}

	if err := os.MkdirAll(fcgiPath, 0700); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to create %s: %s\n", fcgiPath, err)
		os.Exit(1)
	}

	if err := os.Chown(fcgiPath, 48, 48); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to chown %s: %s\n", fcgiPath, err)
		os.Exit(1)
	}

	// Create command.
	cmd := exec.Command(args[1], args[2:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: 48,
			Gid: 48,
		},
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start command.
	if err := cmd.Start(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to start cmd: %s\n", err)
		os.Exit(1)
	}

	// Create a channel for the command.
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
		close(waitCh)
	}()

	// Create a channel for OS signals.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan)

	// Wait for command & proxy signals.
	for {
		select {

		case sig := <-sigChan:
			_ = cmd.Process.Signal(sig)

		case err := <-waitCh:
			var waitStatus syscall.WaitStatus
			if exitError, ok := err.(*exec.ExitError); ok {
				waitStatus = exitError.Sys().(syscall.WaitStatus)
				os.Exit(waitStatus.ExitStatus())
			}
			if err != nil {
				_, _ = fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return

		}
	}
}
