package rpi

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CustomizeImage mounts the boot partition of an uncompressed .img image
// (via `sudo losetup -fP --show` and `sudo mount`), and then applies opts:
//   - EnableSSH: creates an empty `ssh` file,
//   - Wi-Fi: writes a classic wpa_supplicant.conf (country, ssid, psk),
//   - Username/Password: writes userconf.txt as "user:hash"
//     (hashed via `openssl passwd -6`),
//   - Hostname: mounts the root partition (p2) if possible and updates
//     /etc/hostname and /etc/hosts; if mounting fails, it skips this step.
//
// Every sudo command is logged via log() before execution.
// Cleanup (umount, losetup -d) is always deferred.
func CustomizeImage(img string, opts CustomizeOpts, log func(string)) error {
	logf := func(s string) {
		if log != nil {
			log(s)
		}
	}
	if isCompressedName(img) || !strings.HasSuffix(strings.ToLower(img), ".img") {
		return fmt.Errorf("only uncompressed .img images can be customized (got: %s)", filepath.Base(img))
	}

	// Attach the image as a loop device with partition detection.
	out, err := runLogged(log, "sudo", "losetup", "-fP", "--show", img)
	if err != nil {
		return fmt.Errorf("cannot attach image as loop device: %w", err)
	}
	loop := strings.TrimSpace(out)
	if loop == "" {
		return fmt.Errorf("losetup did not return a loop device name for %s", filepath.Base(img))
	}
	defer func() {
		if _, err := runLogged(log, "sudo", "losetup", "-d", loop); err != nil {
			logf("Warning: failed to detach loop device: " + err.Error())
		}
	}()

	bootPart := loop + "p1"
	if err := waitForDevice(bootPart); err != nil {
		return fmt.Errorf("image does not contain a boot partition: %w", err)
	}

	bootDir, err := os.MkdirTemp("", "rpi-boot-")
	if err != nil {
		return fmt.Errorf("cannot create temporary directory: %w", err)
	}
	defer os.Remove(bootDir)

	if _, err := runLogged(log, "sudo", "mount", bootPart, bootDir); err != nil {
		return fmt.Errorf("cannot mount boot partition: %w", err)
	}
	defer func() {
		if _, err := runLogged(log, "sudo", "umount", bootDir); err != nil {
			logf("Warning: failed to unmount boot partition: " + err.Error())
		}
	}()

	// Enable SSH server: an empty `ssh` file on the boot partition is enough.
	if opts.EnableSSH {
		if _, err := runLogged(log, "sudo", "touch", filepath.Join(bootDir, "ssh")); err != nil {
			return fmt.Errorf("cannot enable SSH: %w", err)
		}
		logf("Enabled SSH server.")
	}

	// Wi-Fi configuration in classic wpa_supplicant format.
	if opts.WifiSSID != "" {
		country := opts.WifiCountry
		if country == "" {
			country = "PL"
		}
		conf := fmt.Sprintf(`country=%s
ctrl_interface=DIR=/var/run/wpa_supplicant GROUP=netdev
update_config=1

network={
	ssid=%q
	psk=%q
}
`, country, opts.WifiSSID, opts.WifiPass)
		if err := writeFileSudo(filepath.Join(bootDir, "wpa_supplicant.conf"), conf, log); err != nil {
			return fmt.Errorf("cannot save Wi-Fi configuration: %w", err)
		}
		logf("Saved Wi-Fi configuration for network \"" + opts.WifiSSID + "\".")
	}

	// User account: userconf.txt in "user:hash" format.
	if opts.Password != "" {
		user := opts.Username
		if user == "" {
			user = "pi"
		}
		hash, err := hashPassword(opts.Password, log)
		if err != nil {
			return err
		}
		if err := writeFileSudo(filepath.Join(bootDir, "userconf.txt"), user+":"+hash+"\n", log); err != nil {
			return fmt.Errorf("cannot save userconf.txt file: %w", err)
		}
		logf("Configured user account \"" + user + "\".")
	}

	// Hostname: requires system partition (p2) — this is an optional step.
	if opts.Hostname != "" {
		if err := setHostname(loop, opts.Hostname, log); err != nil {
			logf("Skipping hostname setting: " + err.Error())
		} else {
			logf("Set hostname \"" + opts.Hostname + "\".")
		}
	}

	return nil
}

// hashPassword hashes the password with SHA-512 crypt via
// `openssl passwd -6`. The password is passed on stdin so it does not
// appear in the process list or log.
func hashPassword(password string, log func(string)) (string, error) {
	if _, err := exec.LookPath("openssl"); err != nil {
		return "", fmt.Errorf("missing 'openssl' program in system: %w", err)
	}
	if log != nil {
		log("$ openssl passwd -6 -stdin")
	}
	cmd := exec.Command("openssl", "passwd", "-6", "-stdin")
	cmd.Stdin = strings.NewReader(password + "\n")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("hashing password failed: %w", err)
	}
	hash := strings.TrimSpace(string(out))
	if hash == "" {
		return "", fmt.Errorf("openssl returned empty password hash")
	}
	return hash, nil
}

// setHostname mounts the root partition (p2) and writes the new
// hostname to /etc/hostname and updates 127.0.1.1 in /etc/hosts.
func setHostname(loop, hostname string, log func(string)) error {
	rootPart := loop + "p2"
	if err := waitForDevice(rootPart); err != nil {
		return fmt.Errorf("image does not contain a system partition: %w", err)
	}
	rootDir, err := os.MkdirTemp("", "rpi-root-")
	if err != nil {
		return fmt.Errorf("cannot create temporary directory: %w", err)
	}
	defer os.Remove(rootDir)

	if _, err := runLogged(log, "sudo", "mount", rootPart, rootDir); err != nil {
		return fmt.Errorf("cannot mount system partition: %w", err)
	}
	defer func() {
		if _, err := runLogged(log, "sudo", "umount", rootDir); err != nil && log != nil {
			log("Warning: failed to unmount system partition: " + err.Error())
		}
	}()

	if err := writeFileSudo(filepath.Join(rootDir, "etc", "hostname"), hostname+"\n", log); err != nil {
		return fmt.Errorf("cannot save /etc/hostname: %w", err)
	}

	// Update 127.0.1.1 entry in /etc/hosts (or append it at the end).
	hostsPath := filepath.Join(rootDir, "etc", "hosts")
	hosts, err := runLogged(log, "sudo", "cat", hostsPath)
	if err != nil {
		return fmt.Errorf("cannot read /etc/hosts: %w", err)
	}
	lines := strings.Split(strings.TrimRight(hosts, "\n"), "\n")
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "127.0.1.1") {
			lines[i] = "127.0.1.1\t" + hostname
			replaced = true
		}
	}
	if !replaced {
		lines = append(lines, "127.0.1.1\t"+hostname)
	}
	if err := writeFileSudo(hostsPath, strings.Join(lines, "\n")+"\n", log); err != nil {
		return fmt.Errorf("cannot save /etc/hosts: %w", err)
	}
	return nil
}
