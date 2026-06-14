// Package esp wraps esptool and mpremote (from tools/.venv) and handles
// port detection and hardware reset of ESP boards.
package esp

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"

	"espflasher/internal/archive"
	"espflasher/internal/paths"
)

// Port is a detected serial port.
type Port struct {
	Device      string
	Description string
	Bridge      string // known USB-UART bridge (CP210x, CH340...)
	LikelyESP   bool
}

// SupportedChips — supported chip families.
var SupportedChips = []string{"esp32", "esp8266", "esp32s2", "esp32s3", "esp32c3", "esp32c6"}

type vidpid struct {
	vid, pid string // pid == "" means any
}

var knownBridges = map[vidpid]string{
	{"10C4", "EA60"}: "CP210x (Silicon Labs)",
	{"1A86", "7523"}: "CH340",
	{"1A86", "55D4"}: "CH9102",
	{"0403", ""}:     "FTDI",
	{"303A", ""}:     "Espressif USB",
}

// ListPorts returns serial ports, ESP-like at the beginning of the list.
func ListPorts() ([]Port, error) {
	details, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, fmt.Errorf("cannot read port list: %w", err)
	}
	var out []Port
	for _, d := range details {
		p := Port{Device: d.Name, Description: d.Product}
		if d.IsUSB {
			vid, pid := strings.ToUpper(d.VID), strings.ToUpper(d.PID)
			if name, ok := knownBridges[vidpid{vid, pid}]; ok {
				p.Bridge, p.LikelyESP = name, true
			} else if name, ok := knownBridges[vidpid{vid, ""}]; ok {
				p.Bridge, p.LikelyESP = name, true
			}
			if p.Description == "" {
				p.Description = "USB " + vid + ":" + pid
			}
		}
		out = append(out, p)
	}
	// ESP-like first
	for i, j := 0, 0; j < len(out); j++ {
		if out[j].LikelyESP {
			out[i], out[j] = out[j], out[i]
			i++
		}
	}
	return out, nil
}

// EsptoolPath looks for esptool: first in tools/.venv, then in PATH.
func EsptoolPath() (string, error) {
	venv := filepath.Join(paths.ToolsDir(), ".venv", "bin")
	for _, name := range []string{"esptool", "esptool.py"} {
		p := filepath.Join(venv, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("esptool not found — run tools/install.sh")
}

// MpremotePath looks for mpremote analogously to esptool.
func MpremotePath() (string, error) {
	p := filepath.Join(paths.ToolsDir(), ".venv", "bin", "mpremote")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("mpremote"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("mpremote not found — run tools/install.sh")
}

var (
	etV5   bool
	etOnce sync.Once
)

// etCmd returns the esptool command name matching the version: v5 uses dashes
// (chip-id, erase-flash, write-flash), v4 uses underscores.
func etCmd(et, name string) string {
	etOnce.Do(func() {
		out, err := exec.Command(et, "version").CombinedOutput()
		if err == nil {
			if m := regexp.MustCompile(`v(\d+)\.`).FindSubmatch(out); m != nil {
				etV5 = string(m[1]) >= "5"
			}
		}
	})
	if etV5 {
		return strings.ReplaceAll(name, "_", "-")
	}
	return name
}

var chipRe = regexp.MustCompile(`(?i)(?:Chip is|Detecting chip type\.+\s*|Connected to)\s*(ESP[0-9A-Za-z-]+)`)

// NormalizeChip maps the full chip name to the family (esp32, esp32s3...).
func NormalizeChip(raw string) string {
	s := strings.ToUpper(raw)
	switch {
	case strings.HasPrefix(s, "ESP8266"):
		return "esp8266"
	case strings.HasPrefix(s, "ESP32-S2"):
		return "esp32s2"
	case strings.HasPrefix(s, "ESP32-S3"):
		return "esp32s3"
	case strings.HasPrefix(s, "ESP32-C3"):
		return "esp32c3"
	case strings.HasPrefix(s, "ESP32-C6"):
		return "esp32c6"
	case strings.HasPrefix(s, "ESP32"):
		return "esp32"
	}
	return strings.ToLower(raw)
}

// DetectChip runs esptool chip_id and returns the chip family.
func DetectChip(port string, log func(string)) (string, error) {
	et, err := EsptoolPath()
	if err != nil {
		return "", err
	}
	var detected string
	err = runStream(func(line string) {
		log(line)
		if m := chipRe.FindStringSubmatch(line); m != nil {
			detected = NormalizeChip(m[1])
		}
	}, et, "--port", port, etCmd(et, "chip_id"))
	if detected == "" {
		if err != nil {
			return "", fmt.Errorf("detection failed: %w", err)
		}
		return "", fmt.Errorf("chip not recognized in esptool output")
	}
	return detected, nil
}

// Reset performs a hardware reset via DTR/RTS lines (EN to low state).
func Reset(port string) error {
	p, err := serial.Open(port, &serial.Mode{BaudRate: 115200})
	if err != nil {
		return fmt.Errorf("cannot open port %s: %w", port, err)
	}
	defer p.Close()
	if err := p.SetDTR(false); err != nil {
		return err
	}
	if err := p.SetRTS(true); err != nil { // EN -> GND
		return err
	}
	time.Sleep(120 * time.Millisecond)
	if err := p.SetRTS(false); err != nil { // EN -> 3V3, start
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return nil
}

// EraseFlash erases the entire flash memory.
func EraseFlash(port string, log func(string)) error {
	et, err := EsptoolPath()
	if err != nil {
		return err
	}
	return runStream(log, et, "--port", port, etCmd(et, "erase_flash"))
}

// FlashBin writes a .bin file at the given offset.
func FlashBin(port, chip, bin, offset string, log func(string)) error {
	et, err := EsptoolPath()
	if err != nil {
		return err
	}
	args := []string{"--port", port, "--baud", "460800"}
	if chip != "" {
		args = append([]string{"--chip", chip}, args...)
	}
	args = append(args, etCmd(et, "write_flash"), "-z", offset, bin)
	return runStream(log, et, args...)
}

// UploadFiles copies files to the board via mpremote (requires MicroPython).
func UploadFiles(port string, files []string, log func(string)) error {
	mp, err := MpremotePath()
	if err != nil {
		return err
	}
	for _, f := range files {
		log("Copying " + filepath.Base(f) + " ...")
		if err := runStream(log, mp, "connect", port, "fs", "cp", f, ":"+filepath.Base(f)); err != nil {
			return fmt.Errorf("copying %s failed: %w", f, err)
		}
	}
	log("Restarting board...")
	_ = runStream(log, mp, "connect", port, "reset")
	return nil
}

var percentRe = regexp.MustCompile(`\((\d{1,3})\s*%\)`)

// ParsePercent extracts the progress percentage from an esptool line ("Writing at ... (42 %)").
func ParsePercent(line string) (int, bool) {
	if m := percentRe.FindStringSubmatch(line); m != nil {
		var v int
		fmt.Sscanf(m[1], "%d", &v)
		return v, true
	}
	return 0, false
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]|\r`)

// cleanLine removes ANSI sequences (esptool cursor/colors) from a log line.
func cleanLine(s string) string {
	return strings.TrimRight(ansiRe.ReplaceAllString(s, ""), " ")
}

// runStream runs a command and passes combined stdout+stderr line by line.
func runStream(log func(string), name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = nil
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		pw.Close()
		return err
	}
	done := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			if line := cleanLine(sc.Text()); line != "" {
				log(line)
			}
		}
		close(done)
	}()
	err := cmd.Wait()
	pw.Close()
	<-done
	if err != nil {
		return fmt.Errorf("%s failed: %w", filepath.Base(name), err)
	}
	return nil
}

// --- MicroPython Firmware ---

type fwInfo struct {
	url    string
	offset string
	prefix string // prefix of the file name, to recognize manually uploaded bins
}

var firmware = map[string]fwInfo{
	"esp32":   {"https://micropython.org/resources/firmware/ESP32_GENERIC-20260406-v1.28.0.bin", "0x1000", "ESP32_GENERIC-"},
	"esp32s2": {"https://micropython.org/resources/firmware/ESP32_GENERIC_S2-20260406-v1.28.0.bin", "0x1000", "ESP32_GENERIC_S2-"},
	"esp32s3": {"https://micropython.org/resources/firmware/ESP32_GENERIC_S3-20260406-v1.28.0.bin", "0x0", "ESP32_GENERIC_S3-"},
	"esp32c3": {"https://micropython.org/resources/firmware/ESP32_GENERIC_C3-20260406-v1.28.0.bin", "0x0", "ESP32_GENERIC_C3-"},
	"esp32c6": {"https://micropython.org/resources/firmware/ESP32_GENERIC_C6-20260406-v1.28.0.bin", "0x0", "ESP32_GENERIC_C6-"},
	"esp8266": {"https://micropython.org/resources/firmware/ESP8266_GENERIC-20260406-v1.28.0.bin", "0x0", "ESP8266_GENERIC-"},
}

// FirmwareOffset returns the flash offset for the chip family.
func FirmwareOffset(chip string) string {
	if fw, ok := firmware[chip]; ok {
		return fw.offset
	}
	return "0x0"
}

// EnsureFirmware returns the path to the MicroPython bin for the chip — existing
// file from firmware/ or downloaded from micropython.org.
func EnsureFirmware(chip string, log func(string)) (string, error) {
	fw, ok := firmware[chip]
	if !ok {
		return "", fmt.Errorf("unsupported chip: %s", chip)
	}
	dir := paths.FirmwareDir()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), fw.prefix) && strings.HasSuffix(e.Name(), ".bin") {
			log("Found firmware: " + e.Name())
			return filepath.Join(dir, e.Name()), nil
		}
	}
	dst := filepath.Join(dir, filepath.Base(fw.url))
	log("Downloading " + fw.url + " ...")
	if err := download(fw.url, dst); err != nil {
		return "", fmt.Errorf("download failed (%w) — drop a .bin manually into firmware/ (prefix %s)", err, fw.prefix)
	}
	log("Saved " + dst)
	return dst, nil
}

// FlashMicroPython writes the MicroPython firmware (erases flash, then write_flash).
func FlashMicroPython(port, chip string, log func(string)) error {
	bin, err := EnsureFirmware(chip, log)
	if err != nil {
		return err
	}
	log("Erasing flash memory...")
	if err := EraseFlash(port, log); err != nil {
		return err
	}
	log("Flashing MicroPython...")
	return FlashBin(port, chip, bin, FirmwareOffset(chip), log)
}

func download(url, dst string) error {
	if err := archive.ValidateURL(url); err != nil {
		return err
	}
	client := archive.SecureClient(10 * time.Minute)
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	return os.Rename(tmp, dst)
}

// HelloWorldPy — minimal test program flashed by the "Hello World" option.
const HelloWorldPy = `# Hello World — flashed by ESP Flasher
import time
try:
    from machine import Pin
    led = Pin(2, Pin.OUT)
except Exception:
    led = None

i = 0
while True:
    i += 1
    print("Hello World from ESP! #", i)
    if led:
        led.value(i % 2)
    time.sleep(1)
`

// FlashHelloWorld writes the built-in hello world as main.py on the board.
func FlashHelloWorld(port string, log func(string)) error {
	tmp, err := os.MkdirTemp("", "esphello")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	f := filepath.Join(tmp, "main.py")
	if err := os.WriteFile(f, []byte(HelloWorldPy), 0o644); err != nil {
		return err
	}
	return UploadFiles(port, []string{f}, log)
}
