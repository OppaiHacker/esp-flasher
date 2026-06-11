# Hello World for ESP32 (MicroPython)
# Prints a greeting every 1 second and flashes the built-in LED.

import time

# Try to initialize the built-in LED (GPIO 2 on most ESP32 boards).
# Some boards (e.g. some ESP32-C3/S2) don't have an LED on this pin,
# so we use try/except - the program will also work without an LED.
led = None
try:
    from machine import Pin
    led = Pin(2, Pin.OUT)
except Exception:
    print("Warning: no built-in LED - continuing without blinking")

counter = 0

while True:
    counter += 1
    # EXPLOSION! easter egg
    print("Hello World from ESP32! EXPLOSION! by OppaiHacker (iteration: {})".format(counter))

    # Blink LED if available
    if led:
        led.value(1)   # turn on LED
        time.sleep(0.1)
        led.value(0)   # turn off LED

    # Wait until full second
    time.sleep(0.9 if led else 1.0)
