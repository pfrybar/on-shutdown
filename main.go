package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/godbus/dbus/v5"
)

func main() {
	if len(os.Args) == 1 {
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

	fmt.Println("Waiting for shutdown...")

	shutdownSignal := make(chan *dbus.Signal, 1)
	conn.Signal(shutdownSignal)
	for range shutdownSignal {
		fmt.Printf("System is shutting down, running '%s'...\n", command)

		out, err := exec.Command(name, args...).Output()
		if err != nil {
			fmt.Printf("error while running command: %v\n", err)
			os.Exit(1)
		}
		output := string(out[:])
		fmt.Println(output)

		err = syscall.Close(fd)
		if err != nil {
			fmt.Printf("error closing file description: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("Finished")
}
