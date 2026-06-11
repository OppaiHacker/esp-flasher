// ESP Flasher — terminal programmer for ESP32/ESP8266 + Raspberry Pi images.
// Author: OppaiHacker
//
// Run TUI:               ./espflasher
// Monitor (subcommand):  ./espflasher monitor --port /dev/ttyUSB0 [--baud 115200]
//
// If there is no access to serial ports, the program restarts itself
// via sudo (permanent fix without sudo: tools/install-udev.sh).
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"espflasher/internal/monitor"
	"espflasher/internal/ui"
)

// elevateIfNeeded checks serial port access; if ports exist but cannot be
// opened, it restarts the program via sudo.
func elevateIfNeeded() {
	if os.Geteuid() == 0 {
		return
	}
	var devs []string
	for _, pat := range []string{"/dev/ttyUSB*", "/dev/ttyACM*"} {
		found, _ := filepath.Glob(pat)
		devs = append(devs, found...)
	}
	if len(devs) == 0 {
		return // no boards — sudo not needed
	}
	for _, d := range devs {
		f, err := os.OpenFile(d, os.O_RDWR, 0)
		if err == nil {
			f.Close()
			return // we have access
		}
	}
	sudo, err := exec.LookPath("sudo")
	if err != nil {
		fmt.Fprintln(os.Stderr, "No serial port access and no sudo found.")
		fmt.Fprintln(os.Stderr, "Fix permissions: tools/install-udev.sh")
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	fmt.Println("No serial port access — restarting via sudo...")
	fmt.Println("(permanent fix without sudo: tools/install-udev.sh)")
	args := append([]string{"sudo", "--preserve-env=TERM,TERMINAL,HOME", exe}, os.Args[1:]...)
	if err := syscall.Exec(sudo, args, os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, "sudo failed:", err)
	}
}

func main() {
	elevateIfNeeded()

	if len(os.Args) > 1 && os.Args[1] == "monitor" {
		fs := flag.NewFlagSet("monitor", flag.ExitOnError)
		port := fs.String("port", "", "serial port, e.g. /dev/ttyUSB0")
		baud := fs.Int("baud", 115200, "baud rate")
		_ = fs.Parse(os.Args[2:])
		if *port == "" {
			fmt.Fprintln(os.Stderr, "monitor: --port is required")
			os.Exit(2)
		}
		if err := monitor.Run(*port, *baud); err != nil {
			fmt.Fprintln(os.Stderr, "monitor:", err)
			os.Exit(1)
		}
		return
	}

	p := tea.NewProgram(ui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "TUI error:", err)
		os.Exit(1)
	}
}
