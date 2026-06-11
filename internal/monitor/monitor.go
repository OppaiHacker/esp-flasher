// Package monitor: interactive serial monitor and launching it
// in a new terminal window.
package monitor

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"go.bug.st/serial"
)

// Run starts the monitor in the current terminal: receives data from the port,
// sends lines typed on stdin to the board (CRLF terminated). ":q" quits.
func Run(port string, baud int) error {
	p, err := serial.Open(port, &serial.Mode{BaudRate: baud})
	if err != nil {
		return fmt.Errorf("cannot open port %s: %w", port, err)
	}
	defer p.Close()

	fmt.Printf("=== Serial monitor %s @ %d ===\n", port, baud)
	fmt.Println("Type a command and press Enter to send to ESP. \":q\" quits.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	// Receive: port -> stdout
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := p.Read(buf)
			if err != nil {
				fmt.Println("\n[monitor] port closed:", err)
				os.Exit(0)
			}
			if n > 0 {
				os.Stdout.Write(buf[:n])
			}
		}
	}()

	// Transmit: stdin -> port
	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()

	for {
		select {
		case <-sig:
			fmt.Println("\n[monitor] done.")
			return nil
		case line, ok := <-lines:
			if !ok || line == ":q" {
				fmt.Println("[monitor] done.")
				return nil
			}
			if _, err := p.Write([]byte(line + "\r\n")); err != nil {
				return fmt.Errorf("send error: %w", err)
			}
		}
	}
}

// terminals in order of preference: name -> arguments before the command
var terminals = []struct {
	bin  string
	args []string
}{
	{"kitty", []string{"--detach"}},
	{"alacritty", []string{"-e"}},
	{"foot", []string{}},
	{"wezterm", []string{"start", "--"}},
	{"gnome-terminal", []string{"--"}},
	{"konsole", []string{"-e"}},
	{"xfce4-terminal", []string{"-x"}},
	{"mate-terminal", []string{"-e"}},
	{"tilix", []string{"-e"}},
	{"terminator", []string{"-x"}},
	{"sakura", []string{"-e"}},
	{"lxterminal", []string{"-e"}},
	{"st", []string{"-e"}},
	{"urxvt", []string{"-e"}},
	{"terminology", []string{"-e"}},
	{"xterm", []string{"-e"}},
}

// termArgs returns the invocation arguments for a given terminal emulator.
// Checks the basename in the list of known terminals, defaults to -e.
func termArgs(name string) []string {
	base := filepath.Base(name)
	for _, t := range terminals {
		if t.bin == base {
			return t.args
		}
	}
	return []string{"-e"}
}

// SpawnTerminal opens a new terminal window with the monitor for the given port.
// Uses $TERMINAL if set, otherwise tries known emulators.
func SpawnTerminal(port string, baud int) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	prog := []string{self, "monitor", "--port", port, "--baud", fmt.Sprint(baud)}

	if t := os.Getenv("TERMINAL"); t != "" {
		if path, err := exec.LookPath(t); err == nil {
			args := termArgs(t)
			return start(path, append(append([]string{}, args...), prog...))
		}
	}
	for _, t := range terminals {
		path, err := exec.LookPath(t.bin)
		if err != nil {
			continue
		}
		return start(path, append(append([]string{}, t.args...), prog...))
	}
	return fmt.Errorf("no terminal emulator found — using built-in monitor")
}

func start(bin string, args []string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot run %s: %w", bin, err)
	}
	go cmd.Wait() // don't leave zombies
	return nil
}
