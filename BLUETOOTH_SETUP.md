# Bluetooth Setup Guide for ESP32 Parking System

## Overview
The parking system now uses a Go-based Bluetooth bridge to communicate between the ESP32 and the backend server. This eliminates the need for Python dependencies and integrates OTP authentication directly into the system.

## System Architecture
```
ESP32 (Bluetooth) <-> Go Bluetooth Bridge <-> Backend HTTP API
```

## Setup Instructions

### 1. Pair ESP32 with Linux System

First, make sure Bluetooth is enabled on your Linux system:
```bash
sudo systemctl start bluetooth
sudo systemctl enable bluetooth
```

Install bluez tools if not already installed:
```bash
sudo apt-get install bluez bluez-tools rfcomm
```

Pair the ESP32 device:
```bash
bluetoothctl
# In bluetoothctl:
power on
scan on
# Wait for ESP32_Parking to appear
pair <ESP32_MAC_ADDRESS>
trust <ESP32_MAC_ADDRESS>
connect <ESP32_MAC_ADDRESS>
exit
```

### 2. Create Serial Port Binding

Bind the Bluetooth device to a serial port:
```bash
sudo rfcomm bind /dev/rfcomm0 <ESP32_MAC_ADDRESS> 1
```

To make this permanent, add to `/etc/systemd/system/rfcomm.service`:
```ini
[Unit]
Description=RFCOMM service
After=bluetooth.service
Requires=bluetooth.service

[Service]
ExecStart=/usr/bin/rfcomm watch /dev/rfcomm0 1 <ESP32_MAC_ADDRESS> 1
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable the service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable rfcomm.service
sudo systemctl start rfcomm.service
```

### 3. Configure Backend

Set the Bluetooth serial port in your environment (optional, defaults to `/dev/rfcomm0`):
```bash
export BT_SERIAL_PORT=/dev/rfcomm0
```

Or add to your `.env` file in `backend-go/`:
```
BT_SERIAL_PORT=/dev/rfcomm0
```

### 4. Run the Backend

Install Go dependencies:
```bash
cd backend-go
go mod download
```

Build and run:
```bash
go build -o mdp-ir-car-parking
./mdp-ir-car-parking
```

The backend will automatically attempt to connect to the ESP32 via Bluetooth on startup.

## Communication Protocol

### ESP32 → Backend
- `UPDATE:spot:occupied` - Occupancy status update (spot 1-3, occupied 0/1)
- `GATE:spot:state` - Gate state notification (state: OPEN/CLOSED)
- `STATUS:...` - Status information

### Backend → ESP32
- `spot:code` - Display OTP on LCD (e.g., "1:1234" for spot 1, code 1234)
- `OPEN:spot` - Open gate after successful OTP validation
- `toggle spot` - Manual gate toggle command (legacy)

## OTP System

### How It Works

1. **User reserves a spot** via the mobile app or web interface
   - Backend generates a 4-digit OTP valid for 15 minutes
   - OTP is sent to the user's device
   - OTP is displayed on the ESP32 LCD via Bluetooth: `spot:code`

2. **LCD Display**
   - Normal mode: Shows gate status (G1:O G2:C G3:*)
   - OTP mode: Shows "Spot X OTP: Code: XXXX" for 30 seconds
   - After 30 seconds or when used, returns to normal display

3. **User enters OTP** in the app to open the gate
   - App sends OTP to backend: `POST /api/parks/park-1/spots/{spotID}/validate-otp`
   - Backend validates OTP (checks expiry and correctness)
   - If valid, backend sends `OPEN:spot` command to ESP32
   - OTP is consumed (single-use)
   - Gate opens automatically

4. **Gate Access**
   - User can toggle gate closed manually using physical buttons
   - Or use the mobile app to toggle gate state

## API Endpoints

### Reserve a Spot
```bash
POST /api/parks/park-1/reserve
Response: {
  "parkId": "park-1",
  "spotId": "park-1-spot-1",
  "spotNumber": 1,
  "otp": "1234",
  "otpExpiry": "2024-02-12T12:30:00Z"
}
```

### Validate OTP and Open Gate
```bash
POST /api/parks/park-1/spots/park-1-spot-1/validate-otp
Content-Type: application/json

{
  "otp": "1234"
}

Response: {
  "ok": true,
  "message": "Gate opened successfully"
}
```

### Get All Parks/Spots
```bash
GET /api/parks
Response: [
  {
    "id": "park-1",
    "name": "Parking Site 1",
    "spots": [
      {
        "id": "park-1-spot-1",
        "number": 1,
        "occupied": true,
        "otp": "",
        "otpExpiry": "0001-01-01T00:00:00Z"
      },
      ...
    ]
  }
]
```

## Troubleshooting

### Bluetooth Connection Issues

Check if ESP32 is paired:
```bash
bluetoothctl devices
```

Check if rfcomm is bound:
```bash
ls -l /dev/rfcomm0
```

Test serial communication:
```bash
sudo screen /dev/rfcomm0 115200
# Type commands: status, toggle 1, etc.
```

### Backend Not Connecting

Check backend logs for Bluetooth connection errors:
```
Attempting to connect to ESP32 on /dev/rfcomm0...
```

If connection fails, the backend will continue running without Bluetooth integration. You can still use the HTTP API, but ESP32 commands won't work.

### Permission Issues

Grant your user access to the serial port:
```bash
sudo usermod -a -G dialout $USER
# Log out and back in for changes to take effect
```

Or run the backend with sudo (not recommended for production):
```bash
sudo ./mdp-ir-car-parking
```

## Testing

### Test ESP32 Commands via Backend

Using curl:
```bash
# Reserve a spot (generates OTP and displays on LCD)
curl -X POST http://localhost:8080/api/parks/park-1/reserve

# Validate OTP and open gate
curl -X POST http://localhost:8080/api/parks/park-1/spots/park-1-spot-1/validate-otp \
  -H "Content-Type: application/json" \
  -d '{"otp":"1234"}'
```

### Manual Testing via Serial Terminal

Connect to ESP32 directly:
```bash
sudo screen /dev/rfcomm0 115200
```

Send commands:
- `status` - Get occupancy status
- `toggle 1` - Toggle gate 1
- `1:1234` - Display OTP "1234" for spot 1
- `OPEN:1` - Open gate 1

ESP32 will respond with status messages.

## Notes

- OTPs are valid for 15 minutes after generation
- OTPs are single-use and automatically cleared after validation
- LCD shows OTP for 30 seconds before returning to status display
- Gates can be manually toggled using physical buttons on ESP32
- Ultrasonic sensors automatically detect occupancy (threshold: 40cm)
- System uses 1 park with 3 spots matching ESP32 hardware configuration
