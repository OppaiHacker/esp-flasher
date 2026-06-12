# ESP Flasher

Terminal **ESP32 / ESP8266** programmer + tool for preparing and flashing OS images on **Raspberry Pi** and similar SBCs. Full TUI (bubbletea) with animations, progress bars, and live log view.
Created by **OppaiHacker**.

## Quick Start

```bash
./tools/install.sh      # venv with esptool/mpremote + Go build
./espflasher            # run TUI
```

Requirements: Go ≥ 1.21, Python 3 (for esptool/mpremote), Linux.
Serial port access: add yourself to the `uucp` (Arch) / `dialout` (Debian) group.

## Features

| Option | Description |
|---|---|
| Detect / select port | scans USB-UART (CP210x, CH340, CH9102, FTDI, native Espressif USB) |
| Detect chip type | `esptool chip_id` — esp32, esp8266, s2, s3, c3, c6 |
| Reset ESP | hardware reset via DTR/RTS lines |
| Flash MicroPython | downloads firmware from micropython.org for detected chip and flashes |
| Hello World | test `main.py` blinking LED (requires MicroPython) |
| Serial Monitor | opens a **new terminal window** — output view + sending commands |
| Flash project | list of projects from `projects/`, type `micropython` (mpremote) or `bin` (esptool); projects can also be installed from a URL (plain file or archive) |
| Erase flash | `erase_flash` with confirmation |
| Raspberry Pi | image download, decompression (.xz/.gz/.zip), customization (hostname, SSH, WiFi, user), `dd` write to SD card, sha256 verification |
| Bootloaders | flash `.bin` files from `bootloaders/`, search bootloaders on GitHub (repositories → release files), download from URL — plain `.bin` or archive (zip/rar/7z/gz/xz/bz2/tar) |

## Directory Structure

```
esp/
├── espflasher          # binary (after compilation)
├── main.go             # entry point: TUI or `monitor` subcommand
├── internal/
│   ├── ui/             # TUI (bubbletea): menus, forms, progress
│   ├── esp/            # esptool/mpremote, ports, reset, MicroPython firmware
│   ├── projects/       # project.json manifests + install from URL
│   ├── bootloaders/    # bootloaders/ dir, GitHub search, URL install
│   ├── archive/        # download + extraction (zip/rar/7z/gz/xz/bz2/tar)
│   ├── monitor/        # serial monitor + opening new terminal
│   ├── rpi/            # images: download, customize, dd, verify
│   └── paths/          # project directories
├── tools/              # tools: install.sh, .venv (esptool, mpremote)
├── projects/           # user projects — see below
│   ├── project1_hello_world/
│   ├── project2_blink/
│   ├── project3_wifi_scanner/
│   └── project4_web_led/
├── firmware/           # downloaded MicroPython bins (cache)
├── bootloaders/        # bootloader .bin files (local + downloaded)
├── iso/                # OS images for Raspberry Pi
└── logs/
```

## Custom Project

Folder in `projects/` with `project.json` manifest:

```json
{
  "name": "My project",
  "description": "What it does",
  "type": "micropython",
  "chips": ["esp32", "esp32c3"],
  "main": "main.py",
  "files": ["main.py", "config.py"]
}
```

Type `bin` (ready firmware):

```json
{ "name": "X", "type": "bin", "bin": "firmware.bin", "flash_offset": "0x10000", "chips": ["esp32"] }
```

## Monitor as Subcommand

```bash
./espflasher monitor --port /dev/ttyUSB0 --baud 115200
```

Typed line + Enter → send to ESP, `:q` quits.

## ⚠️ RASPBERRY PI / ISO FEATURES WARNING ⚠️

THE FEATURES RELATED TO RASPBERRY PI AND ISO (DOWNLOADING, CUSTOMIZING, AND FLASHING IMAGES) HAVE NOT YET BEEN TESTED AND THEIR FUNCTIONALITY IS NOT GUARANTEED. USE THEM AT YOUR OWN RISK.

- Image flashing and customization uses `sudo` (`losetup`, `mount`, `dd`) — every command is logged before execution.
- The media list shows **only removable drives** (lsblk `RM=1`); writing requires typing `YES`.
- Keep images in `iso/`.

## License

This project is licensed under the **GNU General Public License v3.0 (GPL-3.0)** - see the [LICENSE](LICENSE) file for details.

**What does this mean?**
- ✅ You can use, modify, and distribute this software for free.
- ✅ You can use it privately or commercially.
- ❗ If you distribute modified versions of this software, you must also share the source code under the same GPL-3.0 license.
- ❗ This software is provided "as is", without warranty of any kind.
