# ESP32 Bluetooth Connection Guide

## Overview
The ESP32 parking system uses Bluetooth Serial connectivity to communicate with your laptop. The laptop runs a Python bridge that forwards events to the Go backend HTTP API. WiFi has been removed to reduce code size.

**Important:** You must run the Python bridge (`bluetooth_controller.py`) for the backend integration to work. The bridge acts as a translator between ESP32 Bluetooth and the Go backend HTTP API.

## Features
- **Automatic Discovery**: ESP32 advertises as "ESP32_Parking"
- **Bi-directional Communication**: Send commands and receive status updates
- **Toggle Gates**: Gates toggle open/closed on each command (not timed)
- **Real-time LCD Display**: Shows gate status (O=Open, C=Closed, *=Occupied)
- **Multiple Control Methods**:
  - Bluetooth commands via Python bridge
  - Manual buttons on hardware
  - Backend TUI (via Python bridge)

## Setup Instructions

### 1. Upload Code to ESP32
1. Open Arduino IDE
2. Install required libraries:
   - ESP32Servo (via Library Manager)
   - LiquidCrystal_I2C (via Library Manager)
   - BluetoothSerial (built-in with ESP32 core)
3. Upload `sketch_feb12a_copy_20260212020616.ino` to your ESP32
4. The ESP32 will start advertising as "ESP32_Parking"

**Note:** WiFi has been removed to reduce sketch size. Communication is Bluetooth-only.

### 2. Connect from Your Laptop

#### Option A: Using the Python Bridge (Recommended)
```bash
# Install dependencies
pip install pybluez requests

# Run the bridge
python3 bluetooth_controller.py
```

The bridge will:
- Automatically discover the ESP32
- Connect via Bluetooth
- Provide an interactive command interface
- Forward ESP32 events to Go backend HTTP API
- Forward backend commands to ESP32

**Commands:**
- `1`, `2`, `3` - Toggle gate 1, 2, or 3 (open/close)
- `status` or `s` - Show current parking status
- `quit` or `q` - Exit

**Note:** Gates toggle on each command - pressing again will close an open gate.

#### Option B: Using the Shell Script (Linux)
```bash
# Make sure Bluetooth is enabled
sudo systemctl start bluetooth

# Run the connection script
./connect_bluetooth.sh
```

#### Option C: Manual Connection (Linux)

1. **Scan for ESP32:**
   ```bash
   bluetoothctl
   scan on
   # Wait until you see ESP32_Parking
   scan off
   ```

2. **Pair and Connect:**
   ```bash
   pair <MAC_ADDRESS>
   trust <MAC_ADDRESS>
   connect <MAC_ADDRESS>
   ```

3. **Use a Serial Terminal:**
   ```bash
   # Using rfcomm
   sudo rfcomm bind 0 <MAC_ADDRESS> 1
   screen /dev/rfcomm0 115200
   
   # Or using minicom
   minicom -D /dev/rfcomm0 -b 115200
   ```

#### Option D: Using Bluetooth Apps
- **Windows**: Use apps like "Bluetooth Terminal" from Microsoft Store
- **macOS**: Use apps like "Serial" or "CoolTerm"
- **Linux**: Use apps like "Blueman" or "GNOME Bluetooth"

## Available Commands via Bluetooth

Once connected, you can send these commands:

| Command | Description |
|---------|-------------|
| `1`, `2`, `3` | Toggle gate for parking spot 1, 2, or 3 (open ↔ closed) |
| `status` or `s` | Get current parking status (occupancy & distances) |

**Note:** Each command toggles the gate - if it's open, it closes; if it's closed, it opens.

## Automatic Connection

The ESP32 will automatically accept connections from paired devices. To set up auto-connection:

### Linux
Add to `/etc/systemd/system/esp32-parking.service`:
```ini
[Unit]
Description=ESP32 Parking Bluetooth Connection
After=bluetooth.target

[Service]
ExecStart=/usr/bin/python3 /path/to/bluetooth_controller.py
Restart=always
User=your_username

[Install]
WantedBy=multi-user.target
```

Then enable:
```bash
sudo systemctl enable esp32-parking.service
sudo systemctl start esp32-parking.service
```

### Windows
1. Pair the ESP32 in Windows Settings
2. Create a shortcut to `bluetooth_controller.py` in the Startup folder
3. Folder location: `%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup`

### macOS
Create a Launch Agent in `~/Library/LaunchAgents/com.parking.bluetooth.plist`:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.parking.bluetooth</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/bin/python3</string>
        <string>/path/to/bluetooth_controller.py</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>
```

## Troubleshooting

### Cannot Find ESP32
- Ensure ESP32 is powered on and running
- Check Serial Monitor (115200 baud) for "Bluetooth started" message
- Restart Bluetooth: `sudo systemctl restart bluetooth` (Linux)
- Make sure ESP32 is not already connected to another device

### Connection Drops
- Check power supply to ESP32
- Reduce distance between laptop and ESP32
- Avoid physical obstructions

### Gates Not Opening
- Check that servos are properly connected to pins 25, 26, 27
- Verify servo power supply is adequate
- Check Serial Monitor for error messages
- Gates toggle - send the command again to close an open gate

### LCD Display
**Line 1:** Gate status - `G1:O G2:C G3:*`
- `O` = Gate Open
- `C` = Gate Closed  
- `*` = Spot Occupied (car present)

**Line 2:** Distance readings in cm

### Python Script Issues
```bash
# Install pybluez on Linux
sudo apt-get install bluetooth libbluetooth-dev
pip install pybluez requests

# On some systems you may need:
pip install pybluez[ble]
```

## System Architecture

```
┌──────────────┐         Bluetooth        ┌──────────────┐
│   Laptop     │◄─────────────────────────►│    ESP32     │
│              │                            │              │
│ - Python     │                            │ - Sensors    │
│   Bridge     │         HTTP API          │ - Servos     │
│              │◄────────────────►          │ - LCD        │
│ - Backend    │                            │              │
│   (Go)       │                            │              │
└──────────────┘                            └──────────────┘

Flow: ESP32 ←BT→ Python Bridge ←HTTP→ Go Backend
```

## Security Notes

- Bluetooth Serial has no encryption by default
- For production use, implement:
  - PIN/password authentication
  - Encrypted Bluetooth (BLE with bonding)
  - Command validation and rate limiting
- Current implementation is suitable for local/development use

## See Also

- [Backend TUI Documentation](backend-go/tui/README.md)
- [ESP32 Hardware Setup](esp32/README.md)
- [API Documentation](backend-go/README.md)
