// Package i18n handles interface translations (English / Polish).
package i18n

import "sync"

// Lang is the language code.
type Lang string

const (
	PL Lang = "pl"
	EN Lang = "en"
)

var (
	mu  sync.RWMutex
	cur = EN
)

// Set configures the current language.
func Set(l Lang) {
	mu.Lock()
	defer mu.Unlock()
	if l == PL {
		cur = PL
	} else {
		cur = EN
	}
}

// Get returns the current language.
func Get() Lang {
	mu.RLock()
	defer mu.RUnlock()
	return cur
}

// T returns the translation for the key in the current language.
// Fallback: English, then Polish, then the key itself.
func T(key string) string {
	mu.RLock()
	l := cur
	mu.RUnlock()
	if l == PL {
		if s, ok := pl[key]; ok {
			return s
		}
	}
	if s, ok := en[key]; ok {
		return s
	}
	if s, ok := pl[key]; ok {
		return s
	}
	return key
}

var pl = map[string]string{
	"subtitle": "Programator ESP32 / ESP8266 / Raspberry Pi",
	"booting":  "inicjalizacja...",

	"hdr.port":   "Port: ",
	"hdr.chip":   "   Chip: ",
	"hdr.noport": "nie wybrano",
	"hdr.nochip": "nieznany",

	"menu.main":     "Menu główne",
	"menu.rpi":      "Raspberry Pi / obrazy systemów",
	"menu.settings": "Ustawienia",

	"m.port":       "Wykryj / wybierz port",
	"m.port.d":     "skanuje porty USB-UART",
	"m.chip":       "Wykryj typ układu",
	"m.chip.d":     "esptool chip-id",
	"m.reset":      "Reset ESP",
	"m.reset.d":    "sprzętowy reset przez DTR/RTS",
	"m.mpy":        "Wgraj firmware MicroPython",
	"m.mpy.d":      "pobiera i flashuje firmware dla układu",
	"m.hello":      "Wgraj Hello World",
	"m.hello.d":    "testowy main.py (wymaga MicroPythona)",
	"m.monitor":    "Monitor szeregowy",
	"m.monitor.d":  "nowe okno terminala, podgląd + wysyłanie komend",
	"m.project":    "Wgraj projekt",
	"m.project.d":  "lista projektów z projekty/",
	"m.erase":      "Wymaż flash",
	"m.erase.d":    "esptool erase-flash",
	"m.rpi":        "Raspberry Pi / karty SD",
	"m.rpi.d":      "obrazy systemów: pobieranie, customizacja, wgrywanie",
	"m.settings":   "Ustawienia",
	"m.settings.d": "język, prędkość monitora",
	"m.quit":       "Wyjście",

	"rpi.show":     "Pokaż obrazy",
	"rpi.show.d":   "zawartość katalogu iso/",
	"rpi.dl":       "Pobierz obraz",
	"rpi.dl.d":     "popularne systemy albo własny URL",
	"rpi.unpack":   "Rozpakuj obraz",
	"rpi.unpack.d": ".xz / .gz / .zip → .img",
	"rpi.cust":     "Customizuj obraz",
	"rpi.cust.d":   "hostname, SSH, WiFi, użytkownik",
	"rpi.flash":    "Wgraj obraz na nośnik",
	"rpi.flash.d":  "zapis dd na kartę SD / pendrive",
	"rpi.verify":   "Weryfikuj zapis",
	"rpi.verify.d": "porównanie sha256 obrazu i nośnika",
	"rpi.back":     "Powrót",

	"set.lang":   "Język / Language",
	"set.lang.d": "wybór zapamiętywany w config.json",
	"set.baud":   "Prędkość monitora",
	"set.baud.d": "baud rate monitora szeregowego",
	"set.back":   "Powrót",

	"help.menu":    "↑/↓ wybór · Enter zatwierdź · klawisz numeru działa od razu · Ctrl+C wyjście",
	"help.chooser": "↑/↓ wybór · Enter zatwierdź · Esc powrót",
	"help.form":    "Enter dalej · Esc anuluj",
	"help.confirm": "Esc anuluj",
	"help.op.run":  "operacja w toku — proszę czekać",
	"help.op.done": "Enter / Esc — powrót",

	"st.running": " w toku...",
	"st.ok":      "✔ Zakończono pomyślnie",
	"st.err":     "✘ Błąd: ",

	"fl.port_first":  "Najpierw wybierz port (opcja 1).",
	"fl.port_sel":    "Wybrano port ",
	"fl.monitor_on":  "Monitor uruchomiony w nowym oknie",
	"fl.monitor_err": "Monitor: ",
	"fl.cancelled":   "Anulowano (wpisz dokładnie %s aby potwierdzić).",
	"fl.lang_saved":  "Język zapisany: polski",
	"fl.baud_saved":  "Zapisano prędkość monitora: ",
	"fl.no_ports":    "Brak portów szeregowych. Podłącz płytkę i spróbuj ponownie.",
	"fl.no_projects": "Brak projektów w projekty/ (folder z project.json).",
	"fl.no_images":   "Brak pasujących obrazów w iso/ — najpierw pobierz lub rozpakuj.",
	"fl.no_drives":   "Brak nośników wymiennych. Włóż kartę SD / pendrive.",

	"ch.port":       "Wybierz port szeregowy",
	"ch.port.esp":   " — wygląda na ESP",
	"ch.project":    "Wybierz projekt do wgrania",
	"ch.proj.warn":  "  ⚠ nie deklaruje ",
	"ch.images":     "Obrazy w iso/",
	"ch.unpack":     "Który obraz rozpakować?",
	"ch.cust":       "Który obraz customizować? (tylko .img)",
	"ch.flash":      "Który obraz wgrać?",
	"ch.verify":     "Który obraz porównać? (tylko .img)",
	"ch.vdrive":     "Z którym nośnikiem porównać?",
	"ch.drive":      "Wybierz nośnik docelowy (UWAGA: dane zostaną nadpisane!)",
	"ch.popular":    "Pobierz obraz systemu",
	"ch.url":        "Własny URL...",
	"ch.url.d":      "podaj adres obrazu ręcznie",
	"ch.lang":       "Wybierz język / Choose language",
	"ch.compressed": "  [skompresowany]",

	"op.detect":   "Wykrywanie układu",
	"op.reset":    "Reset ESP",
	"op.mpy":      "Wgrywanie MicroPython",
	"op.hello":    "Hello World",
	"op.erase":    "Wymazywanie flash",
	"op.project":  "Wgrywanie projektu: ",
	"op.dl":       "Pobieranie obrazu",
	"op.unpack":   "Rozpakowywanie obrazu",
	"op.cust":     "Customizacja obrazu",
	"op.flashimg": "Wgrywanie obrazu na ",
	"op.verify":   "Weryfikacja zapisu",

	"cf.word":      "TAK",
	"cf.erase":     "Wymazanie całej pamięci flash",
	"cf.flash":     "Wgranie obrazu — WSZYSTKIE dane na nośniku zostaną usunięte",
	"cf.image":     "Obraz: ",
	"cf.drive":     "Nośnik: ",
	"cf.type":      "Aby potwierdzić wpisz ",
	"cf.and_enter": " i Enter: ",
	"cf.typehint":  "wpisz TAK",

	"fm.cust":     "Customizacja obrazu (puste pole = pomiń)",
	"fm.hostname": "Hostname",
	"fm.ssh":      "Włączyć SSH? (t/n)",
	"fm.ssid":     "WiFi SSID",
	"fm.wpass":    "WiFi hasło",
	"fm.country":  "Kraj WiFi",
	"fm.user":     "Użytkownik",
	"fm.upass":    "Hasło użytkownika",
	"fm.url":      "Pobieranie obrazu",
	"fm.url.f":    "URL obrazu",
	"fm.baud":     "Prędkość monitora",
	"fm.baud.f":   "baud (np. 115200)",
	"fm.skipped":  "(pominięto)",

	"log.detect_first": "Najpierw wykrywam typ układu...",
	"log.detected":     "Wykryto układ: ",
	"log.reset_via":    "Reset przez DTR/RTS na ",
	"log.reset_done":   "Płytka zrestartowana.",
	"log.hello":        "Wgrywanie main.py (wymaga MicroPythona na płytce)...",
	"log.mpy_proj":     "Projekt MicroPython — kopiowanie plików przez mpremote.",
	"log.mpy_hint":     "(wymaga firmware MicroPython na płytce — opcja 4 w menu)",
	"log.saved":        "Zapisano: ",
	"log.done":         "Gotowe: ",
	"log.sums_ok":      "Sumy kontrolne zgodne ✔",

	"err.sums":      "sumy kontrolne RÓŻNE — zapis uszkodzony",
	"err.no_chip":   "projekt nie obsługuje układu ",
	"err.proj_type": "nieznany typ projektu: ",
	"err.no_proj":   "Nie znaleziono projektu: ",

	"hint.perm1": "⚠ Brak uprawnień do portu! Dodaj się do grupy i przeloguj:",
	"hint.perm2": "   sudo usermod -aG uucp $USER  (Arch)  /  dialout  (Debian)",
	"hint.perm3": "   Szybka łatka na teraz: sudo chmod a+rw ",

	"mon.title":       "Monitor szeregowy",
	"mon.hint":        "Enter = wyślij • Esc = zamknij",
	"mon.placeholder": "wpisz komendę...",
	"mon.closed":      "Monitor zamknięty",
	"mon.opened":      "Podłączono ",
	"mon.fallback":    "Brak emulatora terminala — otwarto wbudowany monitor",
}

var en = map[string]string{
	"subtitle": "ESP32 / ESP8266 / Raspberry Pi programmer",
	"booting":  "initializing...",

	"hdr.port":   "Port: ",
	"hdr.chip":   "   Chip: ",
	"hdr.noport": "not selected",
	"hdr.nochip": "unknown",

	"menu.main":     "Main menu",
	"menu.rpi":      "Raspberry Pi / OS images",
	"menu.settings": "Settings",

	"m.port":       "Detect / select port",
	"m.port.d":     "scans USB-UART ports",
	"m.chip":       "Detect chip type",
	"m.chip.d":     "esptool chip-id",
	"m.reset":      "Reset ESP",
	"m.reset.d":    "hardware reset via DTR/RTS",
	"m.mpy":        "Flash MicroPython firmware",
	"m.mpy.d":      "downloads and flashes firmware for the chip",
	"m.hello":      "Flash Hello World",
	"m.hello.d":    "test main.py (requires MicroPython)",
	"m.monitor":    "Serial monitor",
	"m.monitor.d":  "new terminal window, output + sending commands",
	"m.project":    "Flash project",
	"m.project.d":  "project list from projekty/",
	"m.erase":      "Erase flash",
	"m.erase.d":    "esptool erase-flash",
	"m.rpi":        "Raspberry Pi / SD cards",
	"m.rpi.d":      "OS images: download, customize, write",
	"m.settings":   "Settings",
	"m.settings.d": "language, monitor baud rate",
	"m.quit":       "Quit",

	"rpi.show":     "Show images",
	"rpi.show.d":   "contents of iso/",
	"rpi.dl":       "Download image",
	"rpi.dl.d":     "popular systems or custom URL",
	"rpi.unpack":   "Unpack image",
	"rpi.unpack.d": ".xz / .gz / .zip → .img",
	"rpi.cust":     "Customize image",
	"rpi.cust.d":   "hostname, SSH, WiFi, user",
	"rpi.flash":    "Write image to media",
	"rpi.flash.d":  "dd write to SD card / USB stick",
	"rpi.verify":   "Verify write",
	"rpi.verify.d": "sha256 compare of image and media",
	"rpi.back":     "Back",

	"set.lang":   "Język / Language",
	"set.lang.d": "choice persisted in config.json",
	"set.baud":   "Monitor baud rate",
	"set.baud.d": "serial monitor speed",
	"set.back":   "Back",

	"help.menu":    "↑/↓ select · Enter confirm · number key acts instantly · Ctrl+C quit",
	"help.chooser": "↑/↓ select · Enter confirm · Esc back",
	"help.form":    "Enter next · Esc cancel",
	"help.confirm": "Esc cancel",
	"help.op.run":  "operation in progress — please wait",
	"help.op.done": "Enter / Esc — back",

	"st.running": " in progress...",
	"st.ok":      "✔ Completed successfully (Bakuretsu!)",
	"st.err":     "✘ Error: ",

	"fl.port_first":  "Select a port first (option 1).",
	"fl.port_sel":    "Selected port ",
	"fl.monitor_on":  "Monitor started in a new window",
	"fl.monitor_err": "Monitor: ",
	"fl.cancelled":   "Cancelled (type exactly %s to confirm).",
	"fl.lang_saved":  "Language saved: English",
	"fl.baud_saved":  "Monitor baud rate saved: ",
	"fl.no_ports":    "No serial ports. Connect a board and try again.",
	"fl.no_projects": "No projects in projekty/ (folder with project.json).",
	"fl.no_images":   "No matching images in iso/ — download or unpack first.",
	"fl.no_drives":   "No removable media. Insert an SD card / USB stick.",

	"ch.port":       "Select serial port",
	"ch.port.esp":   " — looks like an ESP",
	"ch.project":    "Select project to flash",
	"ch.proj.warn":  "  ⚠ does not declare ",
	"ch.images":     "Images in iso/",
	"ch.unpack":     "Which image to unpack?",
	"ch.cust":       "Which image to customize? (.img only)",
	"ch.flash":      "Which image to write?",
	"ch.verify":     "Which image to compare? (.img only)",
	"ch.vdrive":     "Which media to compare with?",
	"ch.drive":      "Select target media (WARNING: data will be overwritten!)",
	"ch.popular":    "Download OS image",
	"ch.url":        "Custom URL...",
	"ch.url.d":      "enter image address manually",
	"ch.lang":       "Wybierz język / Choose language",
	"ch.compressed": "  [compressed]",

	"op.detect":   "Detecting chip",
	"op.reset":    "ESP reset",
	"op.mpy":      "Flashing MicroPython",
	"op.hello":    "EXPLOSION! (Hello World)",
	"op.erase":    "Erasing flash",
	"op.project":  "Flashing project: ",
	"op.dl":       "Downloading image",
	"op.unpack":   "Unpacking image",
	"op.cust":     "Customizing image",
	"op.flashimg": "Writing image to ",
	"op.verify":   "Verifying write",

	"cf.word":      "YES",
	"cf.erase":     "Erase the entire flash memory",
	"cf.flash":     "Write image — ALL data on the media will be destroyed",
	"cf.image":     "Image: ",
	"cf.drive":     "Media: ",
	"cf.type":      "To confirm type ",
	"cf.and_enter": " and Enter: ",
	"cf.typehint":  "type YES",

	"fm.cust":     "Image customization (empty field = skip)",
	"fm.hostname": "Hostname",
	"fm.ssh":      "Enable SSH? (y/n)",
	"fm.ssid":     "WiFi SSID",
	"fm.wpass":    "WiFi password",
	"fm.country":  "WiFi country",
	"fm.user":     "Username",
	"fm.upass":    "User password",
	"fm.url":      "Download image",
	"fm.url.f":    "Image URL",
	"fm.baud":     "Monitor baud rate",
	"fm.baud.f":   "baud (e.g. 115200)",
	"fm.skipped":  "(skipped)",

	"log.detect_first": "Detecting chip type first...",
	"log.detected":     "Detected chip: ",
	"log.reset_via":    "Reset via DTR/RTS on ",
	"log.reset_done":   "Board restarted.",
	"log.hello":        "Casting EXPLOSION spell... (Uploading main.py)",
	"log.mpy_proj":     "MicroPython project — copying files via mpremote.",
	"log.mpy_hint":     "(requires MicroPython firmware on the board — option 4)",
	"log.saved":        "Saved: ",
	"log.done":         "Done: ",
	"log.sums_ok":      "Checksums match ✔",

	"err.sums":      "checksums DIFFER — write corrupted",
	"err.no_chip":   "project does not support chip ",
	"err.proj_type": "unknown project type: ",
	"err.no_proj":   "Project not found: ",

	"hint.perm1": "⚠ No permission for the port! Add yourself to the group and re-login:",
	"hint.perm2": "   sudo usermod -aG uucp $USER  (Arch)  /  dialout  (Debian)",
	"hint.perm3": "   Quick fix for now: sudo chmod a+rw ",

	"mon.title":       "Serial Monitor",
	"mon.hint":        "Enter = send • Esc = close",
	"mon.placeholder": "type command...",
	"mon.closed":      "Monitor closed",
	"mon.opened":      "Connected to ",
	"mon.fallback":    "No terminal emulator found — using built-in monitor",
}
