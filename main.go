package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

const version = "v0.2.0"

func main() {
	if len(os.Args) == 1 {
		fmt.Printf("version: %s\n", version)
		fmt.Println("usage: on-shutdown <command> [options...]")
		os.Exit(0)
	}
	name := os.Args[1]
	args := os.Args[2:]
	command := strings.Join(append([]string{name}, args...), " ")

	conn, err := dbus.SystemBus()
	if err != nil {
		fmt.Printf("error getting system bus: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	var delayMicros int64
	prop, err := conn.Object(
		"org.freedesktop.login1",
		dbus.ObjectPath("/org/freedesktop/login1"),
	).GetProperty("org.freedesktop.login1.Manager.InhibitDelayMaxUSec")
	if err != nil {
		fmt.Printf("error getting inhibit delay property: %v\n", err)
		os.Exit(1)
	}
	prop.Store(&delayMicros)
	delay := time.Duration(delayMicros * 1000)

	var fd int
	err = conn.Object(
		"org.freedesktop.login1",
		dbus.ObjectPath("/org/freedesktop/login1"),
	).Call(
		"org.freedesktop.login1.Manager.Inhibit", // Method
		0,                                        // Flags
		"shutdown",                               // What
		"on-shutdown",                            // Who
		"Run on shutdown: "+command,              // Why
		"delay",                                  // Mode
	).Store(&fd)
	if err != nil {
		fmt.Printf("error setting up shutdown inhibitor: %v\n", err)
		os.Exit(1)
	}

	err = conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
		dbus.WithMatchObjectPath("/org/freedesktop/login1"),
		dbus.WithMatchMember("PrepareForShutdown"),
	)
	if err != nil {
		fmt.Printf("error adding match signal: %v\n", err)
		os.Exit(1)
	}

	shutdownSignal := make(chan *dbus.Signal, 1)
	conn.Signal(shutdownSignal)

	fmt.Println("Waiting for shutdown...")
	_ = <-shutdownSignal

	fmt.Printf("System is shutting down, running '%s'...\n", command)
	sysChannel := make(chan os.Signal, 1)
	signal.Notify(sysChannel, syscall.SIGTERM)

	start := time.Now()
	cmdChannel := make(chan bool, 1)
	go run(name, args, cmdChannel)

	select {
	case <-sysChannel:
		end := time.Now()
		if end.Sub(start) > delay {
			fmt.Printf(
				"Error: command did not complete within the InhibitDelayMaxSec (%s) and was killed - consider increasing this value: "+
					"https://www.freedesktop.org/software/systemd/man/latest/logind.conf.html#InhibitDelayMaxSec=\n", delay,
			)
		}
	case <-cmdChannel:
		// no-op
	}

	err = syscall.Close(fd)
	if err != nil {
		fmt.Printf("error closing file description: %v\n", err)
		os.Exit(1)
	}
}

func run(name string, args []string, cmdChannel chan<- bool) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		fmt.Printf("error while starting command: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Wait(); err != nil {
		fmt.Printf("error while running command: %v\n", err)
		cmdChannel <- false
	} else {
		fmt.Println("Command completed successfully")
		cmdChannel <- true
	}
}
