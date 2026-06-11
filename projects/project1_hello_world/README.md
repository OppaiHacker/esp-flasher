# Project 1: Hello World

The simplest starter project for ESP32 with MicroPython.

What it does:
- every 1 second prints "Hello World from ESP32! EXPLOSION! by OppaiHacker" to the console along with an iteration counter,
- at each iteration, briefly flashes the built-in LED (GPIO 2).

Connection: requires no additional components - just the board itself.
If the board does not have an LED on GPIO 2, the program continues to run (only prints text).

Modifications: change the pin number in `Pin(2, Pin.OUT)` or the time in `time.sleep(...)`.
