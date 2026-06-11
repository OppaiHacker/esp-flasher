// Package rpi handles OS image management for Raspberry Pi and
// other SBCs: listing and downloading images, decompressing,
// customizing (SSH, Wi-Fi, user, hostname), and flashing to removable media.
package rpi

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Image describes an OS image found in the images directory.
type Image struct {
	Path       string
	Name       string // file name (basename)
	SizeH      string // human-readable size, e.g., "2.4 GB"
	Compressed bool   // whether it is compressed (.xz/.gz/.zip)
}

// Drive describes a removable media drive (e.g., SD card or USB stick).
type Drive struct {
	Device string // device path, e.g., "/dev/sdb"
	SizeH  string
	Model  string
}

// CustomizeOpts contains options for customizing the image before first
// boot (equivalent to settings in Raspberry Pi Imager).
type CustomizeOpts struct {
	Hostname    string
	EnableSSH   bool
	WifiSSID    string
	WifiPass    string
	WifiCountry string // default "US"
	Username    string
	Password    string // plain password; hashed via `openssl passwd -6` into userconf.txt
}

// PopularImage is a popular OS image that can be downloaded from the net.
type PopularImage struct{ Name, URL string }

// PopularImages is a list of popular OS images for Raspberry Pi.
// "_latest" URLs from downloads.raspberrypi.com always point to
// the latest release of the given variant.
var PopularImages = []PopularImage{
	{Name: "Raspberry Pi OS Lite (64-bit)", URL: "https://downloads.raspberrypi.com/raspios_lite_arm64_latest"},
	{Name: "Raspberry Pi OS Desktop (64-bit)", URL: "https://downloads.raspberrypi.com/raspios_arm64_latest"},
	{Name: "Raspberry Pi OS Full (64-bit)", URL: "https://downloads.raspberrypi.com/raspios_full_arm64_latest"},
	{Name: "Raspberry Pi OS Lite (32-bit)", URL: "https://downloads.raspberrypi.com/raspios_lite_armhf_latest"},
	{Name: "DietPi (RPi 1-4, ARMv8)", URL: "https://dietpi.com/downloads/images/DietPi_RPi-ARMv8-Bookworm.img.xz"},
	{Name: "DietPi (RPi 5)", URL: "https://dietpi.com/downloads/images/DietPi_RPi5-ARMv8-Bookworm.img.xz"},
}

// HumanSize formats size in bytes into a human-readable format,
// e.g., "2.4 GB".
func HumanSize(n int64) string {
	if n < 0 {
		return "?"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), units[exp])
}

// runLogged runs an external command, first writing it to the log
// via log(). It returns the combined output of the command.
func runLogged(log func(string), name string, args ...string) (string, error) {
	cmdline := name
	if len(args) > 0 {
		cmdline += " " + strings.Join(args, " ")
	}
	if log != nil {
		log("$ " + cmdline)
	}
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("command %q failed: %w (%s)",
			cmdline, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// writeFileSudo writes content to a file requiring root privileges
// (via `sudo tee`).
func writeFileSudo(path, content string, log func(string)) error {
	if log != nil {
		log("$ sudo tee " + path)
	}
	cmd := exec.Command("sudo", "tee", path)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = io.Discard
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("saving file %s failed: %w (%s)",
			path, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// waitForDevice waits for a device node to appear in /dev (e.g., after
// `losetup -P`, loop partitions can appear with a delay).
func waitForDevice(path string) error {
	for i := 0; i < 30; i++ {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("device %s did not appear in the system", path)
}

// isCompressedName checks if the file is compressed based on its extension.
func isCompressedName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".xz") ||
		strings.HasSuffix(lower, ".gz") ||
		strings.HasSuffix(lower, ".zip")
}
