# Project 2: Blink

Classic LED blinking with easy configuration.

What it does:
- at startup (optionally) sends an SOS signal in Morse code,
- then blinks the LED in a steady rhythm.

Connection: by default uses the built-in LED on GPIO 2. You can connect an
external LED through an approx. 220 ohm resistor between the selected pin and GND.

Modifications: at the top of the `main.py` file, change the constants `PIN`, `INTERVAL`,
and `SOS_AT_START`.
