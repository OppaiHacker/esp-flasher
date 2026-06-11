# Projects — how to create them so ESP Flasher can see and upload them

Each project is a **folder in `projects/`** with a **`project.json`** file (manifest).
A folder without a manifest is skipped in the list.

```
projects/
└── my_project/
    ├── project.json   ← manifest (REQUIRED)
    ├── main.py        ← entry point
    ├── config.py      ← (optional) configuration to edit before upload
    ├── lib_xyz.py     ← (optional) libraries/drivers
    └── README.md      ← (recommended) description and wiring diagram
```

## `project.json` Manifest

### Type `micropython` (.py files copied to the board)

```json
{
  "name": "My project",
  "description": "Short description — visible in the list in flasher",
  "type": "micropython",
  "chips": ["esp32", "esp32c3"],
  "main": "main.py",
  "files": ["main.py", "config.py", "lib_xyz.py"]
}
```

| Field | Meaning |
|---|---|
| `name` | name on the selection list |
| `description` | description on the list (keep it short!) |
| `type` | `micropython` or `bin` |
| `chips` | supported chips: `esp32`, `esp8266`, `esp32s2`, `esp32s3`, `esp32c3`, `esp32c6`; empty list = all |
| `main` | main startup file of the project |
| `files` | **ALL** files copied to the board — what is not listed here will not be uploaded! |

### Type `bin` (precompiled firmware)

```json
{
  "name": "My firmware",
  "description": "Compiled e.g. from ESP-IDF / Arduino",
  "type": "bin",
  "chips": ["esp32"],
  "bin": "firmware.bin",
  "flash_offset": "0x10000"
}
```

`flash_offset` — upload address (default `0x10000`, typical for an application).

## Rules for MicroPython Projects

1. **`main.py` runs automatically** after a board reset — the main loop should be inside it.
2. **MicroPython firmware is required on the board** — first select option 4 in the flasher, then the project.
   Without this, `mpremote` will not connect and the upload will fail.
3. **Only pure MicroPython**: modules `machine`, `network`, `socket`, `time`, `framebuf`...
   No `typing`, no libraries from PyPI. External drivers (e.g., `ssd1306.py`) should be included
   as a project file and added to `files`.
4. **Configuration in `config.py`** (WiFi, pins) — the user edits a single file, not the main code.
5. **Data files** (e.g., `index.html`) must also be added to `files` — you read them on the board
   with a simple `open("index.html")`.
6. **Error handling**: missing sensor/network should not crash the program —
   use `try/except OSError` and output a message via `print()` (visible in the serial monitor).
7. **No non-ASCII characters in comments** in .py files — the serial console encodes them differently.
8. After uploading, the flasher performs a **reset** — the project starts immediately; view output via the
   Serial Monitor (option 6).

## Common Errors

- ✘ file not added to `files` → it's not on the board, `ImportError`
- ✘ project uploaded without MicroPython → `mpremote` timeout
- ✘ `description` taking up 3 lines → breaks the selection list layout
- ✘ heavy work in `main.py` before `wifi_connect()` → looks like a freeze; print status to console/OLED
