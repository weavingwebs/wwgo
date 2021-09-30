package main

import (
	"fmt"
	_ "github.com/joho/godotenv/autoload"
	"os"
	"os/exec"
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

	cmd := exec.Command(args[1], args[2:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: 48,
			Gid: 48,
		},
		Setpgid: true,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		_, _ = fmt.Fprintf(os.Stderr, "Failed to run cmd: %s\n", err)
		os.Exit(1)
	}
}
