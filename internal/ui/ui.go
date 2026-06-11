// Package ui provides the TUI for the programmer (bubbletea).
package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"espflasher/internal/config"
	"espflasher/internal/esp"
	"espflasher/internal/i18n"
	"espflasher/internal/monitor"
	"espflasher/internal/paths"
	"espflasher/internal/projects"
	"espflasher/internal/rpi"
)

// T - shortcut for translations.
var T = i18n.T

type screen int

const (
	scrBoot screen = iota
	scrMenu
	scrRpiMenu
	scrSettings
	scrChooser
	scrForm
	scrConfirm
	scrProgress
	scrMonitor
)

// chooser actions - what to do with the selected item
type action int

const (
	actNone action = iota
	actPort
	actLang
	actProject
	actPopularImage
	actImageShow
	actImageDecompress
	actImageCustomize
	actImageFlash
	actImageVerify
	actDrive
	actDriveVerify
)

// form actions
type formAction int

const (
	formNone formAction = iota
	formCustomize
	formURL
	formBaud
)

// confirmation actions
type confirmAction int

const (
	confirmNone confirmAction = iota
	confirmErase
	confirmFlashImage
)

type chooserItem struct {
	title string
	desc  string
	value string // payload (path, port, chip...)
}

type formField struct {
	label       string
	placeholder string
	secret      bool
	value       string
}

type bootTickMsg struct{}

// Main TUI model.
type Model struct {
	width, height int
	scr           screen
	prevScr       screen // where esc returns from chooser/form
	bootStep      int

	cfg config.Config

	// device state
	port string
	chip string

	// menu
	menuCursor    int
	rpiMenuCursor int
	setMenuCursor int

	// chooser
	chTitle  string
	chItems  []chooserItem
	chCursor int
	chAction action

	// form
	formTitle  string
	formFields []formField
	formIdx    int
	formAction formAction
	input      textinput.Model

	// confirmation
	confTitle  string
	confDetail string
	confAction confirmAction

	// operation in progress
	opTitle    string
	opRunning  bool
	opErr      error
	opLogs     []string
	permHinted bool    // permission hint shown once per operation
	percent    float64 // -1 = indeterminate
	spin       spinner.Model
	prog       progress.Model

	// rpi selections
	selImage string
	selDrive string

	// built-in monitor
	monSession *monitor.Session
	monLines   []string
	monBuf     string

	flash string // short message on menu (e.g. after reset)
}

// New creates the initial model: loads config, sets language, restores port.
func New() Model {
	cfg := config.Load()
	i18n.Set(i18n.Lang(cfg.Lang))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("51"))
	ti := textinput.New()
	ti.CharLimit = 256

	m := Model{
		cfg:     cfg,
		scr:     scrBoot,
		percent: -1,
		spin:    sp,
		prog:    progress.New(progress.WithDefaultGradient()),
		input:   ti,
	}
	// restore last port if still connected
	if cfg.LastPort != "" {
		if ports, err := esp.ListPorts(); err == nil {
			for _, p := range ports {
				if p.Device == cfg.LastPort {
					m.port = cfg.LastPort
					break
				}
			}
		}
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(bootTick(), m.spin.Tick)
}

func bootTick() tea.Cmd {
	return tea.Tick(60*time.Millisecond, func(time.Time) tea.Msg { return bootTickMsg{} })
}

var bannerLines = strings.Split(strings.Trim(banner, "\n"), "\n")
var bannerWidth = lipgloss.Width(banner)

// ---------- UPDATE ----------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.prog.Width = min(60, msg.Width-10)
		return m, nil

	case bootTickMsg:
		if m.scr != scrBoot {
			return m, nil
		}
		m.bootStep++
		if m.bootStep > len(bannerLines)+8 {
			m.scr = scrMenu
			return m, nil
		}
		return m, bootTick()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case chMsg:
		return m.handleOpEvent(msg)

	case serialDataMsg:
		return m.handleSerialData(msg)

	case serialErrMsg:
		return m.handleSerialErr(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.scr == scrForm || m.scr == scrConfirm || m.scr == scrMonitor {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()

	if key == "ctrl+c" {
		m.stopMonitor()
		return m, tea.Quit
	}

	switch m.scr {
	case scrBoot:
		m.scr = scrMenu // any key skips animation
		return m, nil

	case scrMenu:
		entries := mainMenu()
		m.flash = ""
		if navigate(key, &m.menuCursor, len(entries)) {
			return m, nil
		}
		if key == "enter" {
			return m.menuSelect(entries[m.menuCursor].key)
		}
		for i, e := range entries {
			if e.key == key {
				m.menuCursor = i
				return m.menuSelect(key)
			}
		}
		return m, nil

	case scrRpiMenu:
		entries := rpiMenu()
		m.flash = ""
		if key == "esc" {
			m.scr = scrMenu
			return m, nil
		}
		if navigate(key, &m.rpiMenuCursor, len(entries)) {
			return m, nil
		}
		if key == "enter" {
			return m.rpiMenuSelect(entries[m.rpiMenuCursor].key)
		}
		for i, e := range entries {
			if e.key == key {
				m.rpiMenuCursor = i
				return m.rpiMenuSelect(key)
			}
		}
		return m, nil

	case scrSettings:
		entries := settingsMenu()
		m.flash = ""
		if key == "esc" {
			m.scr = scrMenu
			return m, nil
		}
		if navigate(key, &m.setMenuCursor, len(entries)) {
			return m, nil
		}
		if key == "enter" {
			return m.settingsSelect(entries[m.setMenuCursor].key)
		}
		for i, e := range entries {
			if e.key == key {
				m.setMenuCursor = i
				return m.settingsSelect(key)
			}
		}
		return m, nil

	case scrChooser:
		return m.keyChooser(key)

	case scrForm:
		switch key {
		case "esc":
			m.scr = m.prevScr
			return m, nil
		case "enter":
			return m.formNext()
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(k)
			return m, cmd
		}

	case scrConfirm:
		switch key {
		case "esc":
			m.scr = m.prevScr
			return m, nil
		case "enter":
			if strings.TrimSpace(m.input.Value()) == T("cf.word") {
				return m.confirmAccepted()
			}
			m.scr = m.prevScr
			m.flash = styWarn.Render(fmt.Sprintf(T("fl.cancelled"), T("cf.word")))
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(k)
			return m, cmd
		}

	case scrProgress:
		if !m.opRunning && (key == "enter" || key == "esc" || key == "q") {
			m.scr = m.prevScr
			return m, nil
		}
		return m, nil

	case scrMonitor:
		switch key {
		case "esc":
			m.stopMonitor()
			m.scr = m.prevScr
			m.flash = styOK.Render(T("mon.closed"))
			return m, nil
		case "enter":
			line := m.input.Value()
			m.input.SetValue("")
			if m.monSession != nil {
				m.monSession.Send(line)
				m.monLines = append(m.monLines, styKey.Render("→ ")+line)
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(k)
			return m, cmd
		}
	}
	return m, nil
}

// ---------- MENU ----------

type menuEntry struct {
	key   string
	label string
	desc  string
}

func mainMenu() []menuEntry {
	return []menuEntry{
		{"1", T("m.port"), T("m.port.d")},
		{"2", T("m.chip"), T("m.chip.d")},
		{"3", T("m.reset"), T("m.reset.d")},
		{"4", T("m.mpy"), T("m.mpy.d")},
		{"5", T("m.hello"), T("m.hello.d")},
		{"6", T("m.monitor"), T("m.monitor.d")},
		{"7", T("m.project"), T("m.project.d")},
		{"8", T("m.erase"), T("m.erase.d")},
		{"9", T("m.rpi"), T("m.rpi.d")},
		{"s", T("m.settings"), T("m.settings.d")},
		{"q", T("m.quit"), ""},
	}
}

func rpiMenu() []menuEntry {
	return []menuEntry{
		{"1", T("rpi.show"), T("rpi.show.d")},
		{"2", T("rpi.dl"), T("rpi.dl.d")},
		{"3", T("rpi.unpack"), T("rpi.unpack.d")},
		{"4", T("rpi.cust"), T("rpi.cust.d")},
		{"5", T("rpi.flash"), T("rpi.flash.d")},
		{"6", T("rpi.verify"), T("rpi.verify.d")},
		{"b", T("rpi.back"), ""},
	}
}

func settingsMenu() []menuEntry {
	return []menuEntry{
		{"1", T("set.lang"), ""},
		{"2", T("set.baud"), ""},
		{"b", T("set.back"), ""},
	}
}

// navigate moves the cursor with arrows / j,k. Returns true if key was handled.
// The pointer must point to the model field that will be returned from Update.
func navigate(key string, cursor *int, n int) bool {
	switch key {
	case "up", "k":
		if *cursor > 0 {
			*cursor--
		}
		return true
	case "down", "j":
		if *cursor < n-1 {
			*cursor++
		}
		return true
	}
	return false
}

func (m Model) menuSelect(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "1":
		return m.openPortChooser()
	case "2":
		if !m.requirePort() {
			return m, nil
		}
		return m.startDetect()
	case "3":
		if !m.requirePort() {
			return m, nil
		}
		port := m.port
		return m.startOpScreen(T("op.reset"), func(emit func(tea.Msg)) error {
			emit(logLine(T("log.reset_via") + port + " ..."))
			if err := esp.Reset(port); err != nil {
				return err
			}
			emit(logLine(T("log.reset_done")))
			return nil
		})
	case "4":
		if !m.requirePort() {
			return m, nil
		}
		port, chip := m.port, m.chip
		return m.startOpScreen(T("op.mpy"), func(emit func(tea.Msg)) error {
			c, err := ensureChip(port, chip, emit)
			if err != nil {
				return err
			}
			return esp.FlashMicroPython(port, c, logTo(emit))
		})
	case "5":
		if !m.requirePort() {
			return m, nil
		}
		port := m.port
		return m.startOpScreen(T("op.hello"), func(emit func(tea.Msg)) error {
			emit(logLine(T("log.hello")))
			return esp.FlashHelloWorld(port, logTo(emit))
		})
	case "6":
		if !m.requirePort() {
			return m, nil
		}
		if err := monitor.SpawnTerminal(m.port, m.cfg.Baud); err != nil {
			// Fallback: built-in TUI monitor
			return m.startBuiltinMonitor()
		}
		m.flash = styOK.Render(T("fl.monitor_on") + " (" + m.port + " @ " + strconv.Itoa(m.cfg.Baud) + ")")
		return m, nil
	case "7":
		if !m.requirePort() {
			return m, nil
		}
		return m.openProjectChooser()
	case "8":
		if !m.requirePort() {
			return m, nil
		}
		m.prevScr = scrMenu
		m.confTitle = T("cf.erase")
		m.confDetail = "Port: " + m.port
		m.confAction = confirmErase
		m.openConfirmInput()
		return m, textinput.Blink
	case "9":
		m.scr = scrRpiMenu
		return m, nil
	case "s":
		m.scr = scrSettings
		return m, nil
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) openConfirmInput() {
	m.input.SetValue("")
	m.input.Placeholder = T("cf.typehint")
	m.input.EchoMode = textinput.EchoNormal
	m.input.Focus()
	m.scr = scrConfirm
}

func (m *Model) requirePort() bool {
	if m.port == "" {
		m.flash = styWarn.Render(T("fl.port_first"))
		return false
	}
	return true
}

func (m Model) rpiMenuSelect(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "1":
		return m.openImageChooser(T("ch.images"), actImageShow, false)
	case "2":
		return m.openPopularChooser()
	case "3":
		return m.openImageChooser(T("ch.unpack"), actImageDecompress, false)
	case "4":
		return m.openImageChooser(T("ch.cust"), actImageCustomize, true)
	case "5":
		return m.openImageChooser(T("ch.flash"), actImageFlash, false)
	case "6":
		return m.openImageChooser(T("ch.verify"), actImageVerify, true)
	case "b":
		m.scr = scrMenu
		return m, nil
	}
	return m, nil
}

func (m Model) settingsSelect(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "1":
		m.prevScr = scrSettings
		return m.openChooser(T("ch.lang"), []chooserItem{
			{title: "Polski", desc: "Polish language", value: "pl"},
			{title: "English", desc: "English language", value: "en"},
		}, actLang)
	case "2":
		m.formTitle = T("fm.baud")
		m.formFields = []formField{{label: T("fm.baud.f"), placeholder: strconv.Itoa(m.cfg.Baud)}}
		m.formIdx = 0
		m.formAction = formBaud
		m.prevScr = scrSettings
		m.setInputForField()
		m.scr = scrForm
		return m, textinput.Blink
	case "b":
		m.scr = scrMenu
		return m, nil
	}
	return m, nil
}

// ---------- CHOOSERY ----------

func (m Model) openChooser(title string, items []chooserItem, act action) (tea.Model, tea.Cmd) {
	if m.scr == scrMenu || m.scr == scrRpiMenu || m.scr == scrSettings {
		m.prevScr = m.scr
	}
	m.chTitle, m.chItems, m.chCursor, m.chAction = title, items, 0, act
	m.scr = scrChooser
	return m, nil
}

func (m Model) openPortChooser() (tea.Model, tea.Cmd) {
	ports, err := esp.ListPorts()
	if err != nil {
		m.flash = styErr.Render(err.Error())
		return m, nil
	}
	if len(ports) == 0 {
		m.flash = styWarn.Render(T("fl.no_ports"))
		return m, nil
	}
	items := make([]chooserItem, len(ports))
	for i, p := range ports {
		desc := p.Description
		if p.Bridge != "" {
			desc += "  [" + p.Bridge + T("ch.port.esp") + "]"
		}
		items[i] = chooserItem{title: p.Device, desc: desc, value: p.Device}
	}
	return m.openChooser(T("ch.port"), items, actPort)
}

func (m Model) openProjectChooser() (tea.Model, tea.Cmd) {
	projs, err := projects.Scan(paths.ProjectsDir())
	if err != nil {
		m.flash = styErr.Render(err.Error())
		return m, nil
	}
	if len(projs) == 0 {
		m.flash = styWarn.Render(T("fl.no_projects"))
		return m, nil
	}
	var items []chooserItem
	for _, p := range projs {
		desc := p.Description + "  [" + p.Type + "; " + strings.Join(p.Chips, ", ") + "]"
		if m.chip != "" && !p.SupportsChip(m.chip) {
			desc += T("ch.proj.warn") + m.chip
		}
		items = append(items, chooserItem{title: p.Name, desc: desc, value: p.Dir})
	}
	return m.openChooser(T("ch.project"), items, actProject)
}

func (m Model) openImageChooser(title string, act action, onlyImg bool) (tea.Model, tea.Cmd) {
	imgs, err := rpi.ListImages(paths.IsoDir())
	if err != nil {
		m.flash = styErr.Render(err.Error())
		return m, nil
	}
	var items []chooserItem
	for _, im := range imgs {
		if onlyImg && im.Compressed {
			continue
		}
		desc := im.SizeH
		if im.Compressed {
			desc += T("ch.compressed")
		}
		items = append(items, chooserItem{title: im.Name, desc: desc, value: im.Path})
	}
	if len(items) == 0 {
		m.flash = styWarn.Render(T("fl.no_images"))
		return m, nil
	}
	return m.openChooser(title, items, act)
}

func (m Model) openPopularChooser() (tea.Model, tea.Cmd) {
	var items []chooserItem
	for _, p := range rpi.PopularImages {
		items = append(items, chooserItem{title: p.Name, desc: p.URL, value: p.URL})
	}
	items = append(items, chooserItem{title: T("ch.url"), desc: T("ch.url.d"), value: ""})
	return m.openChooser(T("ch.popular"), items, actPopularImage)
}

func (m Model) openDriveChooser(act action, title string) (tea.Model, tea.Cmd) {
	drives, err := rpi.ListRemovableDrives()
	if err != nil {
		m.flash = styErr.Render(err.Error())
		m.scr = scrRpiMenu
		return m, nil
	}
	if len(drives) == 0 {
		m.flash = styWarn.Render(T("fl.no_drives"))
		m.scr = scrRpiMenu
		return m, nil
	}
	var items []chooserItem
	for _, d := range drives {
		items = append(items, chooserItem{title: d.Device, desc: d.Model + "  " + d.SizeH, value: d.Device})
	}
	return m.openChooser(title, items, act)
}

func (m Model) keyChooser(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.scr = m.prevScr
		return m, nil
	case "up", "k":
		if m.chCursor > 0 {
			m.chCursor--
		}
	case "down", "j":
		if m.chCursor < len(m.chItems)-1 {
			m.chCursor++
		}
	case "enter":
		return m.chooserSelect(m.chItems[m.chCursor])
	}
	return m, nil
}

func (m Model) chooserSelect(it chooserItem) (tea.Model, tea.Cmd) {
	switch m.chAction {
	case actPort:
		m.port = it.value
		m.chip = "" // new port — unknown chip
		m.cfg.LastPort = it.value
		_ = m.cfg.Save()
		m.scr = scrMenu
		m.flash = styOK.Render(T("fl.port_sel") + it.value)
		return m, nil

	case actLang:
		m.cfg.Lang = it.value
		i18n.Set(i18n.Lang(it.value))
		_ = m.cfg.Save()
		m.scr = scrSettings
		m.flash = styOK.Render(T("fl.lang_saved"))
		return m, nil

	case actProject:
		return m.startFlashProject(it.value)

	case actImageShow:
		return m, nil // view only, esc returns

	case actImageDecompress:
		img := it.value
		return m.startOpScreen(T("op.unpack"), func(emit func(tea.Msg)) error {
			out, err := rpi.Decompress(img, progressTo(emit))
			if err != nil {
				return err
			}
			emit(logLine(T("log.done") + out))
			return nil
		})

	case actImageCustomize:
		m.selImage = it.value
		return m.openCustomizeForm()

	case actImageFlash:
		m.selImage = it.value
		return m.openDriveChooser(actDrive, T("ch.drive"))

	case actImageVerify:
		m.selImage = it.value
		return m.openDriveChooser(actDriveVerify, T("ch.vdrive"))

	case actDriveVerify:
		img, dev := m.selImage, it.value
		return m.startOpScreen(T("op.verify"), func(emit func(tea.Msg)) error {
			ok, err := rpi.VerifyFlash(img, dev, progressTo(emit), logTo(emit))
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("%s", T("err.sums"))
			}
			emit(logLine(T("log.sums_ok")))
			return nil
		})

	case actDrive:
		m.selDrive = it.value
		m.prevScr = scrRpiMenu
		m.confTitle = T("cf.flash")
		m.confDetail = T("cf.image") + m.selImage + "\n" + T("cf.drive") + m.selDrive
		m.confAction = confirmFlashImage
		m.openConfirmInput()
		return m, textinput.Blink

	case actPopularImage:
		if it.value == "" { // custom URL
			m.formTitle = T("fm.url")
			m.formFields = []formField{{label: T("fm.url.f"), placeholder: "https://..."}}
			m.formIdx = 0
			m.formAction = formURL
			m.prevScr = scrRpiMenu
			m.setInputForField()
			m.scr = scrForm
			return m, textinput.Blink
		}
		url := it.value
		return m.startOpScreen(T("op.dl"), func(emit func(tea.Msg)) error {
			path, err := rpi.DownloadImage(url, paths.IsoDir(), progressTo(emit))
			if err != nil {
				return err
			}
			emit(logLine(T("log.saved") + path))
			return nil
		})
	}
	return m, nil
}

// ---------- FORMS ----------

func (m Model) openCustomizeForm() (tea.Model, tea.Cmd) {
	m.formTitle = T("fm.cust")
	m.formFields = []formField{
		{label: T("fm.hostname"), placeholder: "rpi"},
		{label: T("fm.ssh"), placeholder: "t"},
		{label: T("fm.ssid"), placeholder: ""},
		{label: T("fm.wpass"), placeholder: "", secret: true},
		{label: T("fm.country"), placeholder: "PL"},
		{label: T("fm.user"), placeholder: "pi"},
		{label: T("fm.upass"), placeholder: "", secret: true},
	}
	m.formIdx = 0
	m.formAction = formCustomize
	m.prevScr = scrRpiMenu
	m.setInputForField()
	m.scr = scrForm
	return m, textinput.Blink
}

func (m *Model) setInputForField() {
	f := m.formFields[m.formIdx]
	m.input.SetValue("")
	m.input.Placeholder = f.placeholder
	if f.secret {
		m.input.EchoMode = textinput.EchoPassword
	} else {
		m.input.EchoMode = textinput.EchoNormal
	}
	m.input.Focus()
}

func (m Model) formNext() (tea.Model, tea.Cmd) {
	m.formFields[m.formIdx].value = strings.TrimSpace(m.input.Value())
	if m.formIdx < len(m.formFields)-1 {
		m.formIdx++
		m.setInputForField()
		return m, textinput.Blink
	}
	// form finished
	switch m.formAction {
	case formURL:
		url := m.formFields[0].value
		if url == "" {
			m.scr = m.prevScr
			return m, nil
		}
		return m.startOpScreen(T("op.dl"), func(emit func(tea.Msg)) error {
			path, err := rpi.DownloadImage(url, paths.IsoDir(), progressTo(emit))
			if err != nil {
				return err
			}
			emit(logLine(T("log.saved") + path))
			return nil
		})
	case formBaud:
		if v, err := strconv.Atoi(m.formFields[0].value); err == nil && v > 0 {
			m.cfg.Baud = v
			_ = m.cfg.Save()
			m.flash = styOK.Render(T("fl.baud_saved") + strconv.Itoa(v))
		}
		m.scr = scrSettings
		return m, nil
	case formCustomize:
		v := func(i int) string { return m.formFields[i].value }
		sshAns := strings.ToLower(v(1))
		opts := rpi.CustomizeOpts{
			Hostname:    v(0),
			EnableSSH:   strings.HasPrefix(sshAns, "t") || strings.HasPrefix(sshAns, "y"),
			WifiSSID:    v(2),
			WifiPass:    v(3),
			WifiCountry: v(4),
			Username:    v(5),
			Password:    v(6),
		}
		if opts.WifiCountry == "" {
			opts.WifiCountry = "PL"
		}
		img := m.selImage
		return m.startOpScreen(T("op.cust"), func(emit func(tea.Msg)) error {
			return rpi.CustomizeImage(img, opts, logTo(emit))
		})
	}
	m.scr = m.prevScr
	return m, nil
}

// ---------- CONFIRMATIONS ----------

func (m Model) confirmAccepted() (tea.Model, tea.Cmd) {
	switch m.confAction {
	case confirmErase:
		port := m.port
		return m.startOpScreen(T("op.erase"), func(emit func(tea.Msg)) error {
			return esp.EraseFlash(port, logTo(emit))
		})
	case confirmFlashImage:
		img, dev := m.selImage, m.selDrive
		return m.startOpScreen(T("op.flashimg")+dev, func(emit func(tea.Msg)) error {
			return rpi.FlashImage(img, dev, progressTo(emit), logTo(emit))
		})
	}
	m.scr = m.prevScr
	return m, nil
}

// ---------- VIEW ----------

func (m Model) View() string {
	switch m.scr {
	case scrBoot:
		return m.viewBoot()
	case scrMenu:
		return m.viewMenu(T("menu.main"), mainMenu(), m.menuCursor)
	case scrRpiMenu:
		return m.viewMenu(T("menu.rpi"), rpiMenu(), m.rpiMenuCursor)
	case scrSettings:
		return m.viewMenu(T("menu.settings"), m.settingsMenuView(), m.setMenuCursor)
	case scrChooser:
		return m.viewChooser()
	case scrForm:
		return m.viewForm()
	case scrConfirm:
		return m.viewConfirm()
	case scrProgress:
		return m.viewProgress()
	case scrMonitor:
		return m.viewMonitor()
	}
	return ""
}

// settingsMenuView — settings entries with current values.
func (m Model) settingsMenuView() []menuEntry {
	lang := "polski"
	if m.cfg.Lang == "en" {
		lang = "English"
	}
	return []menuEntry{
		{"1", T("set.lang"), T("set.lang.d") + "  [" + lang + "]"},
		{"2", T("set.baud"), T("set.baud.d") + "  [" + strconv.Itoa(m.cfg.Baud) + "]"},
		{"b", T("set.back"), ""},
	}
}

func (m Model) viewBoot() string {
	n := min(m.bootStep, len(bannerLines))
	out := styBanner.Render(strings.Join(bannerLines[:n], "\n"))
	if n == len(bannerLines) {
		out += "\n\n" + stySubtitle.Render(T("subtitle"))
		out += "\n\n" + m.spin.View() + styDim.Render(" "+T("booting"))
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(out)
}

// header — full banner when terminal is wide enough, compact otherwise.
func (m Model) header() string {
	var logo string
	if m.width == 0 || m.width >= bannerWidth+4 {
		logo = styBanner.Render(strings.Join(bannerLines, "\n"))
	} else {
		logo = styBanner.Render("⚡ ESP FLASHER")
	}
	port := styStatusOff.Render(T("hdr.noport"))
	if m.port != "" {
		port = styStatusOn.Render(m.port)
	}
	chip := styWarn.Render(T("hdr.nochip"))
	if m.chip != "" {
		chip = styStatusOn.Render(m.chip)
	}
	status := styDim.Render(T("hdr.port")) + port + styDim.Render(T("hdr.chip")) + chip
	return logo + "\n" + styTitle.Render(T("subtitle")) + "\n" + status + "\n"
}

// headerHeight — number of lines taken by header (to calculate log space).
func (m Model) headerHeight() int {
	if m.width == 0 || m.width >= bannerWidth+4 {
		return len(bannerLines) + 5
	}
	return 6
}

func (m Model) viewMenu(title string, entries []menuEntry, cursor int) string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n" + stySubtitle.Render(title) + "\n\n")
	for i, e := range entries {
		line := styKey.Render("["+e.key+"] ") + e.label
		if e.desc != "" {
			line += "  " + styItemDesc.Render("— "+e.desc)
		}
		if i == cursor {
			b.WriteString(styItemSel.Render("➜ " + line))
		} else {
			b.WriteString(styItem.Render(line))
		}
		b.WriteString("\n")
	}
	if m.flash != "" {
		b.WriteString("\n" + m.flash + "\n")
	}
	b.WriteString(styHelp.Render(T("help.menu")))
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m Model) viewChooser() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n" + stySubtitle.Render(m.chTitle) + "\n\n")
	for i, it := range m.chItems {
		if i == m.chCursor {
			b.WriteString(styItemSel.Render("➜ "+it.title) + "\n   " + styItemDesc.Render(it.desc))
		} else {
			line := it.title
			if it.desc != "" {
				line += "\n   " + styItemDesc.Render(it.desc)
			}
			b.WriteString(styItem.Render(line))
		}
		b.WriteString("\n")
	}
	b.WriteString(styHelp.Render(T("help.chooser")))
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m Model) viewForm() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n" + stySubtitle.Render(m.formTitle) + "\n\n")
	for i := 0; i < m.formIdx; i++ {
		val := m.formFields[i].value
		if m.formFields[i].secret && val != "" {
			val = strings.Repeat("•", len(val))
		}
		if val == "" {
			val = styDim.Render(T("fm.skipped"))
		}
		b.WriteString("  " + styOK.Render("✔ ") + m.formFields[i].label + ": " + val + "\n")
	}
	b.WriteString("\n  " + styKey.Render(m.formFields[m.formIdx].label+": ") + m.input.View() + "\n")
	b.WriteString(styHelp.Render(T("help.form")))
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m Model) viewConfirm() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n" + styErr.Render("⚠ "+m.confTitle) + "\n\n")
	b.WriteString(m.confDetail + "\n\n")
	b.WriteString(T("cf.type") + styErr.Render(T("cf.word")) + T("cf.and_enter") + m.input.View() + "\n")
	b.WriteString(styHelp.Render(T("help.confirm")))
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m Model) viewProgress() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n" + stySubtitle.Render(m.opTitle) + "\n\n")

	if m.opRunning {
		b.WriteString(m.spin.View() + T("st.running") + "\n")
	} else if m.opErr != nil {
		b.WriteString(styErr.Render(T("st.err")+m.opErr.Error()) + "\n")
	} else {
		b.WriteString(styOK.Render(T("st.ok")) + "\n")
	}

	if m.percent >= 0 {
		b.WriteString("\n" + m.prog.ViewAs(m.percent) + "\n")
	}

	// last log lines
	maxLines := 12
	if m.height > 0 {
		maxLines = max(5, m.height-m.headerHeight()-9)
	}
	logs := m.opLogs
	if len(logs) > maxLines {
		logs = logs[len(logs)-maxLines:]
	}
	if len(logs) > 0 {
		w := max(40, m.width-8)
		b.WriteString("\n" + styLogBox.Width(w).Render(styLog.Render(strings.Join(logs, "\n"))) + "\n")
	}

	if !m.opRunning {
		b.WriteString(styHelp.Render(T("help.op.done")))
	} else {
		b.WriteString(styHelp.Render(T("help.op.run")))
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

// ---------- BUILT-IN MONITOR ----------

// startBuiltinMonitor opens port and switches to TUI monitor screen.
func (m Model) startBuiltinMonitor() (tea.Model, tea.Cmd) {
	s, err := monitor.Open(m.port, m.cfg.Baud)
	if err != nil {
		m.flash = styErr.Render(T("fl.monitor_err") + err.Error())
		return m, nil
	}

	m.prevScr = scrMenu
	m.scr = scrMonitor
	m.monSession = s
	m.monLines = []string{
		styOK.Render("=== " + T("mon.title") + " ==="),
		styDim.Render(T("mon.opened") + m.port + " @ " + strconv.Itoa(m.cfg.Baud)),
		styDim.Render(T("mon.fallback")),
		"",
	}
	m.monBuf = ""

	m.input.SetValue("")
	m.input.Placeholder = T("mon.placeholder")
	m.input.EchoMode = textinput.EchoNormal
	m.input.Focus()

	return m, tea.Batch(monitorReadCmd(s), textinput.Blink)
}

// handleSerialData processes raw port data into monLines.
func (m Model) handleSerialData(msg serialDataMsg) (tea.Model, tea.Cmd) {
	if m.scr != scrMonitor || m.monSession == nil {
		return m, nil
	}

	data := m.monBuf + string(msg)
	for {
		idx := strings.IndexAny(data, "\n\r")
		if idx < 0 {
			m.monBuf = data
			break
		}
		line := data[:idx]
		if line != "" {
			m.monLines = append(m.monLines, line)
		}
		// skip \r\n or \n\r
		if idx+1 < len(data) &&
			((data[idx] == '\r' && data[idx+1] == '\n') ||
				(data[idx] == '\n' && data[idx+1] == '\r')) {
			data = data[idx+2:]
		} else {
			data = data[idx+1:]
		}
	}

	// limit buffer
	const maxMonLines = 500
	if len(m.monLines) > maxMonLines {
		m.monLines = m.monLines[len(m.monLines)-maxMonLines:]
	}

	return m, monitorReadCmd(m.monSession)
}

// handleSerialErr handles port error (cable disconnection etc.).
func (m Model) handleSerialErr(msg serialErrMsg) (tea.Model, tea.Cmd) {
	if m.scr != scrMonitor {
		return m, nil
	}
	m.stopMonitor()
	m.scr = m.prevScr
	m.flash = styWarn.Render(T("mon.closed"))
	return m, nil
}

// stopMonitor closes monitor session and clears state.
func (m *Model) stopMonitor() {
	if m.monSession != nil {
		m.monSession.Close()
		m.monSession = nil
	}
}

// viewMonitor renders the built-in monitor screen.
func (m Model) viewMonitor() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n" + stySubtitle.Render(T("mon.title")) + "\n\n")

	// log area
	maxLines := 16
	if m.height > 0 {
		maxLines = max(5, m.height-m.headerHeight()-8)
	}
	logs := m.monLines
	// show partial line if any
	if m.monBuf != "" {
		logs = append(logs, m.monBuf)
	}
	if len(logs) > maxLines {
		logs = logs[len(logs)-maxLines:]
	}
	if len(logs) > 0 {
		w := max(40, m.width-8)
		b.WriteString(styLogBox.Width(w).Render(styLog.Render(strings.Join(logs, "\n"))) + "\n")
	}

	// input field
	b.WriteString("\n" + styKey.Render("→ ") + m.input.View() + "\n")
	b.WriteString(styHelp.Render(T("mon.hint")))
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}
