package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"espflasher/internal/bootloaders"
	"espflasher/internal/esp"
	"espflasher/internal/monitor"
	"espflasher/internal/paths"
	"espflasher/internal/projects"
)

// Background operation messages.
type logLine string
type percentMsg float64
type chipDetectedMsg string
type opDoneMsg struct{ err error }

// chMsg wraps an event from the operation channel along with the channel,
// so Update can continue listening.
type chMsg struct {
	inner tea.Msg
	ch    chan tea.Msg
}

// startOpScreen switches to the progress screen and fires the operation in a goroutine.
func (m Model) startOpScreen(title string, fn func(emit func(tea.Msg)) error) (tea.Model, tea.Cmd) {
	if m.scr == scrMenu || m.scr == scrRpiMenu || m.scr == scrBootMenu || m.scr == scrSettings {
		m.prevScr = m.scr
	}
	m.scr = scrProgress
	m.opTitle = title
	m.opRunning = true
	m.opErr = nil
	m.opLogs = nil
	m.permHinted = false
	m.percent = -1

	ch := make(chan tea.Msg, 256)
	go func() {
		err := fn(func(msg tea.Msg) { ch <- msg })
		ch <- opDoneMsg{err}
		close(ch)
	}()
	return m, tea.Batch(readCh(ch), m.spin.Tick)
}

func readCh(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return chMsg{inner: msg, ch: ch}
	}
}

// permissionProblem detects a serial port access error.
func permissionProblem(s string) bool {
	return strings.Contains(s, "Permission denied") ||
		strings.Contains(s, "Errno 13") ||
		strings.Contains(s, "could not open port")
}

func (m *Model) appendPermHint() {
	if m.permHinted {
		return
	}
	m.permHinted = true
	m.opLogs = append(m.opLogs,
		styWarn.Render(T("hint.perm1")),
		styWarn.Render(T("hint.perm2")),
		styWarn.Render(T("hint.perm3")+m.port),
	)
}

func (m Model) handleOpEvent(ev chMsg) (tea.Model, tea.Cmd) {
	switch msg := ev.inner.(type) {
	case logLine:
		m.opLogs = append(m.opLogs, string(msg))
		if permissionProblem(string(msg)) {
			m.appendPermHint()
		}
		// esptool reports percentages in the log
		if p, ok := esp.ParsePercent(string(msg)); ok {
			m.percent = float64(p) / 100
		}
	case percentMsg:
		m.percent = float64(msg)
	case chipDetectedMsg:
		m.chip = string(msg)
	case opDoneMsg:
		m.opRunning = false
		m.opErr = msg.err
		if msg.err != nil && permissionProblem(msg.err.Error()) {
			m.appendPermHint()
		}
		if msg.err == nil && m.percent >= 0 {
			m.percent = 1
		}
		return m, nil
	}
	return m, readCh(ev.ch)
}

// logTo adapts the log(string) callback to emit TUI messages.
func logTo(emit func(tea.Msg)) func(string) {
	return func(s string) { emit(logLine(s)) }
}

// progressTo adapts the progress(done,total) callback to a progress bar.
func progressTo(emit func(tea.Msg)) func(done, total int64) {
	return func(done, total int64) {
		if total > 0 {
			emit(percentMsg(float64(done) / float64(total)))
		}
	}
}

// ensureChip returns the known chip type or detects it with esptool.
func ensureChip(port, chip string, emit func(tea.Msg)) (string, error) {
	if chip != "" {
		return chip, nil
	}
	emit(logLine(T("log.detect_first")))
	c, err := esp.DetectChip(port, logTo(emit))
	if err != nil {
		return "", err
	}
	emit(chipDetectedMsg(c))
	emit(logLine(T("log.detected") + c))
	return c, nil
}

// startDetect — "detect chip type" operation.
func (m Model) startDetect() (tea.Model, tea.Cmd) {
	port := m.port
	return m.startOpScreen(T("op.detect"), func(emit func(tea.Msg)) error {
		c, err := esp.DetectChip(port, logTo(emit))
		if err != nil {
			return err
		}
		emit(chipDetectedMsg(c))
		emit(logLine(T("log.detected") + c))
		return nil
	})
}

// startFlashProject flashes the selected project (micropython via mpremote,
// bin via esptool).
func (m Model) startFlashProject(dir string) (tea.Model, tea.Cmd) {
	projs, err := projects.Scan(paths.ProjectsDir())
	if err != nil {
		m.flash = styErr.Render(err.Error())
		m.scr = scrMenu
		return m, nil
	}
	var proj *projects.Project
	for i := range projs {
		if projs[i].Dir == dir {
			proj = &projs[i]
			break
		}
	}
	if proj == nil {
		m.flash = styErr.Render(T("err.no_proj") + dir)
		m.scr = scrMenu
		return m, nil
	}
	p := *proj
	port, chip := m.port, m.chip
	return m.startOpScreen(T("op.project")+p.Name, func(emit func(tea.Msg)) error {
		switch p.Type {
		case "micropython":
			emit(logLine(T("log.mpy_proj")))
			emit(logLine(T("log.mpy_hint")))
			return esp.UploadFiles(port, p.FilePaths(), logTo(emit))
		case "bin":
			c, err := ensureChip(port, chip, emit)
			if err != nil {
				return err
			}
			if !p.SupportsChip(c) {
				return fmt.Errorf("%s%s", T("err.no_chip"), c)
			}
			offset := p.FlashOffset
			if offset == "" {
				offset = "0x10000"
			}
			return esp.FlashBin(port, c, p.Dir+"/"+p.Bin, offset, logTo(emit))
		default:
			return fmt.Errorf("%s%q", T("err.proj_type"), p.Type)
		}
	})
}

// startFlashBootloader writes the selected bootloader .bin at the
// chip-specific offset (0x1000 for esp32/s2, 0x0 otherwise).
func (m Model) startFlashBootloader(bin string) (tea.Model, tea.Cmd) {
	port, chip := m.port, m.chip
	return m.startOpScreen(T("op.blflash"), func(emit func(tea.Msg)) error {
		c, err := ensureChip(port, chip, emit)
		if err != nil {
			return err
		}
		off := bootloaders.Offset(c)
		emit(logLine(T("log.bl_offset") + c + ": " + off))
		return esp.FlashBin(port, c, bin, off, logTo(emit))
	})
}

// startInstallBootloader downloads a bootloader (file or archive) from
// a URL into bootloaders/ and lists the .bin files it produced.
func (m Model) startInstallBootloader(url string) (tea.Model, tea.Cmd) {
	return m.startOpScreen(T("op.bldl"), func(emit func(tea.Msg)) error {
		bins, err := bootloaders.Install(url, paths.BootloadersDir(), progressTo(emit), logTo(emit))
		if err != nil {
			return err
		}
		for _, b := range bins {
			emit(logLine(T("log.bl_found") + b))
		}
		emit(logLine(T("log.bl_hint")))
		return nil
	})
}

// startInstallProject downloads a project (file or archive) from a URL
// into projects/ and generates project.json when missing.
func (m Model) startInstallProject(url string) (tea.Model, tea.Cmd) {
	return m.startOpScreen(T("op.projdl"), func(emit func(tea.Msg)) error {
		dir, err := projects.InstallFromURL(url, paths.ProjectsDir(), progressTo(emit), logTo(emit))
		if err != nil {
			return err
		}
		emit(logLine(T("log.installed") + dir))
		emit(logLine(T("log.proj_hint")))
		return nil
	})
}

// ---------- BUILT-IN MONITOR ----------

// serialDataMsg — raw data received from the serial port.
type serialDataMsg string

// serialErrMsg — port error (disconnection, access loss).
type serialErrMsg struct{ err error }

// monChMsg wraps a monitor event from the session channel.
type monChMsg struct {
	inner tea.Msg
	ch    chan tea.Msg
}

// monitorReadCmd creates a Cmd reading from monitor session channels.
func monitorReadCmd(s *monitor.Session) tea.Cmd {
	return func() tea.Msg {
		recv := s.Recv()
		errCh := s.Err()
		if recv == nil && errCh == nil {
			return nil
		}
		select {
		case data, ok := <-recv:
			if !ok {
				return nil
			}
			return serialDataMsg(data)
		case err, ok := <-errCh:
			if !ok {
				return nil
			}
			return serialErrMsg{err}
		}
	}
}
