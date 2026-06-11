// Session: interactive serial port monitoring session
// designed to be embedded in TUI (bubbletea).
package monitor

import (
	"fmt"

	"go.bug.st/serial"
)

// Session represents an active serial monitor session.
// Recv and Err channels provide data to the TUI event loop.
type Session struct {
	port   serial.Port
	sendCh chan string
	recvCh chan string
	errCh  chan error
}

// Open opens a serial port and starts read/write goroutines.
func Open(portName string, baud int) (*Session, error) {
	p, err := serial.Open(portName, &serial.Mode{BaudRate: baud})
	if err != nil {
		return nil, fmt.Errorf("cannot open port %s: %w", portName, err)
	}

	s := &Session{
		port:   p,
		sendCh: make(chan string, 64),
		recvCh: make(chan string, 256),
		errCh:  make(chan error, 1),
	}

	// Receive: port -> recvCh
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := p.Read(buf)
			if err != nil {
				select {
				case s.errCh <- err:
				default:
				}
				return
			}
			if n > 0 {
				s.recvCh <- string(buf[:n])
			}
		}
	}()

	// Transmit: sendCh -> port
	go func() {
		for line := range s.sendCh {
			if _, err := p.Write([]byte(line + "\r\n")); err != nil {
				return
			}
		}
	}()

	return s, nil
}

// Send sends a line of text to the board (adds CRLF).
func (s *Session) Send(line string) {
	if s == nil || s.sendCh == nil {
		return
	}
	select {
	case s.sendCh <- line:
	default:
	}
}

// Recv returns a channel with data received from the port (raw bytes as string).
func (s *Session) Recv() <-chan string {
	if s == nil {
		return nil
	}
	return s.recvCh
}

// Err returns a channel with a read error (e.g., port disconnected).
func (s *Session) Err() <-chan error {
	if s == nil {
		return nil
	}
	return s.errCh
}

// Close closes the port and channels. Safe to call multiple times.
func (s *Session) Close() {
	if s == nil {
		return
	}
	if s.port != nil {
		s.port.Close()
		s.port = nil
	}
	if s.sendCh != nil {
		close(s.sendCh)
		s.sendCh = nil
	}
}
