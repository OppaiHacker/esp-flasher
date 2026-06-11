package ui

import "github.com/charmbracelet/lipgloss"

// ASCII banner shown at startup and in the header.
const banner = `
 _____ ____  ____    _____ _        _    ____  _   _ _____ ____
| ____/ ___||  _ \  |  ___| |      / \  / ___|| | | | ____|  _ \
|  _| \___ \| |_) | | |_  | |     / _ \ \___ \| |_| |  _| | |_) |
| |___ ___) |  __/  |  _| | |___ / ___ \ ___) |  _  | |___|  _ <
|_____|____/|_|     |_|   |_____/_/   \_\____/|_| |_|_____|_| \_\`

const subtitle = "ESP32 / ESP8266 / Raspberry Pi programmer"

var (
	styBanner   = lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)
	stySubtitle = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
	styTitle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true).
			Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("39")).
			Padding(0, 2)
	styItem     = lipgloss.NewStyle().PaddingLeft(2)
	styItemSel  = lipgloss.NewStyle().PaddingLeft(0).Foreground(lipgloss.Color("51")).Bold(true)
	styItemDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styKey      = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)
	styOK       = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
	styErr      = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	styWarn     = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	styDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styLog      = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	styLogBox   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("60")).Padding(0, 1)
	styStatusOn  = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	styStatusOff = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styHelp      = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
)
