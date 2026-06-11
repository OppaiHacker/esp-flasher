# Project 3: WiFi Scanner

Tool for viewing nearby wireless networks.

What it does:
- every 10 seconds it scans the 2.4 GHz band,
- prints a table: SSID, signal strength (RSSI), channel, and security type,
- networks are sorted starting from the strongest signal.

Connection: requires no additional components.

Modifications: change the `INTERVAL` constant at the top of `main.py` to scan
more or less often. View the results on the serial console (REPL).
