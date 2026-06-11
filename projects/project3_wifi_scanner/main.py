# WiFi Scanner (MicroPython)
# Every 10 seconds it scans the surroundings and prints a table of found networks,
# sorted from the strongest signal (RSSI).

import network
import time

INTERVAL = 10  # time between scans in seconds

# Mapping security code to readable text
SECURITY = {
    0: "open",
    1: "WEP",
    2: "WPA-PSK",
    3: "WPA2-PSK",
    4: "WPA/WPA2-PSK",
    5: "WPA2-ENT",
    6: "WPA3-PSK",
    7: "WPA2/WPA3-PSK",
}

def security_name(code):
    """Convert numerical security code to text description."""
    return SECURITY.get(code, "unknown ({})".format(code))

def scan(wlan):
    """Perform a scan and print a table of networks sorted by RSSI."""
    print("EXPLOSION! Scanning... - OppaiHacker")
    try:
        networks = wlan.scan()
    except OSError as e:
        print("Scan error:", e)
        return

    # Sort descending by signal strength (RSSI is negative, larger = better)
    networks.sort(key=lambda s: s[3], reverse=True)

    print("")
    print("Networks found: {}".format(len(networks)))
    print("{:<32} {:>6} {:>6}  {}".format("SSID", "RSSI", "Channel", "Security"))
    print("-" * 64)

    for net in networks:
        # Tuple: (ssid, bssid, channel, RSSI, security, hidden)
        ssid = net[0].decode("utf-8", "ignore") or "<hidden>"
        channel = net[2]
        rssi = net[3]
        sec = security_name(net[4])
        print("{:<32} {:>6} {:>6}  {}".format(ssid[:32], rssi, channel, sec))

    print("-" * 64)

# --- Main Program ---
# Enable WiFi interface in station mode (client)
wlan = network.WLAN(network.STA_IF)
wlan.active(True)

print("WiFi Scanner - new scan every {} seconds".format(INTERVAL))

while True:
    scan(wlan)
    time.sleep(INTERVAL)
