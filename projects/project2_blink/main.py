# Blink - configurable LED blinking (MicroPython)
# Change the constants below to adapt the program to your board.

from machine import Pin
import time

# ====== CONFIGURATION ======
PIN = 2          # GPIO pin number with LED (2 = built-in LED on ESP32)
INTERVAL = 0.5   # on / off time in seconds
SOS_AT_START = True  # whether to send an SOS signal at program start
# ==========================

led = Pin(PIN, Pin.OUT)

# Time unit for Morse code (dot = 1 unit, dash = 3 units)
UNIT = 0.2

def blink(duration):
    """Turn on the LED for the given duration, then turn it off and wait 1 unit."""
    led.value(1)
    time.sleep(duration)
    led.value(0)
    time.sleep(UNIT)

def sos():
    """Send an SOS signal (... --- ...) with the LED."""
    print("EXPLOSION! Sending SOS signal... - OppaiHacker")
    for letter in ("...", "---", "..."):
        for char in letter:
            if char == ".":
                blink(UNIT)        # dot
            else:
                blink(3 * UNIT)    # dash
        time.sleep(2 * UNIT)       # space between letters
    time.sleep(4 * UNIT)           # space after the whole signal

# --- Main Program ---
print("Blink on GPIO pin {}, interval {} s. EXPLOSION!".format(PIN, INTERVAL))

if SOS_AT_START:
    sos()

while True:
    led.value(1)
    time.sleep(INTERVAL)
    led.value(0)
    time.sleep(INTERVAL)
