#!/usr/bin/env bash
# Installs udev rule granting access to ESP USB-UART bridges
# without sudo and without adding to groups. Permanent "Permission denied" fix.
set -euo pipefail

RULES=/etc/udev/rules.d/99-espflasher.rules

sudo tee "$RULES" >/dev/null <<'EOF'
# ESP Flasher — USB-UART bridges of dev boards
# CP210x (Silicon Labs)
SUBSYSTEM=="tty", ATTRS{idVendor}=="10c4", ATTRS{idProduct}=="ea60", MODE="0666"
# CH340 / CH9102 (WCH)
SUBSYSTEM=="tty", ATTRS{idVendor}=="1a86", MODE="0666"
# FTDI
SUBSYSTEM=="tty", ATTRS{idVendor}=="0403", MODE="0666"
# Espressif native USB (ESP32-S2/S3/C3/C6)
SUBSYSTEM=="tty", ATTRS{idVendor}=="303a", MODE="0666"
EOF

sudo udevadm control --reload-rules
sudo udevadm trigger

echo "Installed: $RULES"
echo "Unplug and replug the board — ports will be accessible without sudo."
