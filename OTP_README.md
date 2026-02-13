# ESP32 OTP-Based Parking System

## Overview

This parking management system uses an ESP32 microcontroller with ultrasonic sensors for occupancy detection, servo-controlled gates, and an LCD display. The system has been migrated from QR code authentication to a more secure and user-friendly **4-digit OTP (One-Time Password)** system.

## Key Features

- **3 Parking Spots** with automatic occupancy detection via ultrasonic sensors
- **Servo-controlled gates** with toggle functionality (open/close)
- **16x2 LCD Display** showing real-time gate status and OTP codes
- **OTP Authentication** - 4-digit codes valid for 15 minutes, single-use
- **Bluetooth Communication** - ESP32 communicates with backend via Bluetooth Serial
- **Go Backend** - RESTful API with SQLite database
- **Manual Override** - Physical buttons for manual gate control

## System Architecture

```
┌─────────────┐      Bluetooth      ┌──────────────────┐      HTTP      ┌─────────────┐
│   ESP32     │ ◄─────────────────► │  Go Backend      │ ◄────────────► │  Mobile App │
│             │  Serial Protocol    │  (Bluetooth      │   RESTful API  │  / Web UI   │
│ - Sensors   │                     │   Bridge)        │                │             │
│ - Servos    │                     │ - OTP Generator  │                │             │
│ - LCD       │                     │ - Validator      │                │             │
│ - Buttons   │                     │ - SQLite DB      │                │             │
└─────────────┘                     └──────────────────┘                └─────────────┘
```

## Hardware Setup

### Components
- ESP32 Development Board
- 3x HC-SR04 Ultrasonic Sensors
- 3x SG90 Servo Motors (for gates)
- 16x2 I2C LCD Display (0x27 address)
- 3x Push Buttons (manual control)
- Power supply and wiring

### Pin Configuration

**Ultrasonic Sensors:**
- Spot 1: TRIG=5, ECHO=18
- Spot 2: TRIG=17, ECHO=16
- Spot 3: TRIG=4, ECHO=2

**Servos:**
- Gate 1: Pin 25
- Gate 2: Pin 26
- Gate 3: Pin 27

**Buttons:**
- Button 1: Pin 32 (toggle gate 1)
- Button 2: Pin 33 (toggle gate 2)
- Button 3: Pin 34 (toggle gate 3)

**LCD:**
- I2C Address: 0x27 (16x2 characters)

## Software Setup

### Prerequisites
- Go 1.21+ installed
- Arduino IDE with ESP32 board support
- Linux system with Bluetooth support
- rfcomm tools (`sudo apt-get install bluez bluez-tools rfcomm`)

### Backend Setup

1. Navigate to backend directory:
```bash
cd backend-go
```

2. Install dependencies:
```bash
go mod download
```

3. Create a `.env` file:
```bash
cp .env.example .env
# Edit .env with your configuration
```

4. Build the backend:
```bash
go build -o mdp-ir-car-parking
```

5. Set up Bluetooth (see [BLUETOOTH_SETUP.md](BLUETOOTH_SETUP.md) for detailed instructions):
```bash
# Pair ESP32
bluetoothctl
> pair <ESP32_MAC>
> trust <ESP32_MAC>
> exit

# Bind to serial port
sudo rfcomm bind /dev/rfcomm0 <ESP32_MAC> 1
```

6. Run the backend:
```bash
./mdp-ir-car-parking
```

### ESP32 Setup

1. Open `esp32/sketch_feb12a_copy_20260212020616/sketch_feb12a_copy_20260212020616.ino` in Arduino IDE

2. Install required libraries via Library Manager:
   - LiquidCrystal_I2C
   - ESP32Servo
   - BluetoothSerial (built-in)

3. Configure board:
   - Board: "ESP32 Dev Module"
   - Upload Speed: 115200
   - Port: Select your ESP32 port

4. Upload the sketch to ESP32

5. Monitor Serial output to verify Bluetooth initialization:
```
BT: ESP32_Parking
```

## OTP System Usage

### User Flow

1. **Reserve a Spot**
   ```bash
   POST /api/parks/park-1/reserve
   ```
   Response:
   ```json
   {
     "parkId": "park-1",
     "spotId": "park-1-spot-1",
     "spotNumber": 1,
     "otp": "1234",
     "otpExpiry": "2024-02-12T15:30:00Z"
   }
   ```
   - Backend generates a 4-digit OTP
   - OTP is valid for **15 minutes**
   - OTP is sent to user's app
   - **OTP is displayed on ESP32 LCD** for 30 seconds

2. **LCD Display**
   - Normal mode: `G1:O G2:C G3:*` (O=Open, C=Closed, *=Occupied)
   - OTP mode: 
     ```
     Spot 1 OTP:
     Code: 1234
     ```
   - After 30 seconds, returns to status display

3. **Validate OTP and Open Gate**
   ```bash
   POST /api/parks/park-1/spots/park-1-spot-1/validate-otp
   Content-Type: application/json
   
   {
     "otp": "1234"
   }
   ```
   Response:
   ```json
   {
     "ok": true,
     "message": "Gate opened successfully"
   }
   ```
   - Backend validates OTP (checks expiry and correctness)
   - If valid, sends `OPEN:1` command to ESP32
   - Gate opens automatically
   - **OTP is consumed** (single-use, cannot be reused)

4. **Gate Control**
   - User can toggle gate closed using physical buttons
   - Or use mobile app to toggle gate state

## API Reference

### Parking Endpoints

#### List All Parks
```
GET /api/parks
```
Returns all parks and their spots with current status.

#### Reserve Any Available Spot
```
POST /api/parks/{parkID}/reserve
```
Reserves the first available spot and returns OTP.

#### Reserve Specific Spot
```
POST /api/parks/{parkID}/spots/{spotID}/reserve
```
Reserves a specific spot and returns OTP.

#### Validate OTP and Open Gate
```
POST /api/parks/{parkID}/spots/{spotID}/validate-otp
Content-Type: application/json

{
  "otp": "1234"
}
```
Validates OTP and opens the gate if valid.

#### Release Spot
```
POST /api/parks/{parkID}/spots/{spotID}/release
```
Releases a reserved spot and clears OTP.

#### Update Spot Occupancy (Internal)
```
POST /api/parks/{parkID}/spots/{spotID}/update
Content-Type: application/json

{
  "occupied": true
}
```
Used by ESP32 bridge to update occupancy status.

### Authentication Endpoints

The system also includes OAuth2 authentication endpoints:
- `GET /login` - Initiate OAuth login
- `GET /callback` - OAuth callback
- `POST /auth/mobile` - Mobile authentication
- `GET /protected` - Protected resource example

## Communication Protocol

### ESP32 → Backend (via Bluetooth)

| Message | Format | Description |
|---------|--------|-------------|
| Occupancy Update | `UPDATE:spot:occupied` | spot=1-3, occupied=0/1 |
| Gate State | `GATE:spot:state` | state=OPEN/CLOSED |
| Status | `STATUS:...` | General status info |

### Backend → ESP32 (via Bluetooth)

| Command | Format | Description |
|---------|--------|-------------|
| Display OTP | `spot:code` | e.g., "1:1234" for spot 1 |
| Open Gate | `OPEN:spot` | Open gate after OTP validation |
| Toggle Gate | `toggle spot` | Manual gate toggle (legacy) |
| Get Status | `status` | Request status update |

## LCD Display Modes

### Normal Mode (Default)
```
G1:O G2:C G3:*
45cm 38cm 15cm
```
- Line 1: Gate status (O=Open, C=Closed, *=Occupied)
- Line 2: Distance readings from ultrasonic sensors

### OTP Mode (30 seconds)
```
Spot 1 OTP:
Code: 1234
```
- Displayed when new OTP is generated
- Automatically returns to normal mode after 30 seconds
- Cleared when OTP is used or expires

## Security Features

1. **Time-Limited OTPs**: Valid for 15 minutes only
2. **Single-Use**: OTP is consumed immediately after validation
3. **Spot-Specific**: Each OTP is tied to a specific parking spot
4. **Automatic Expiry**: Expired OTPs are automatically rejected
5. **Display Timeout**: LCD clears OTP after 30 seconds to prevent shoulder surfing

## Troubleshooting

### ESP32 Issues

**LCD not displaying:**
- Check I2C address (use I2C scanner sketch)
- Verify SDA/SCL connections
- Check 5V power supply

**Sensors not detecting:**
- Verify TRIG/ECHO pin connections
- Check sensor power (5V)
- Adjust `EMPTY_THRESHOLD` if needed (default 40cm)

**Bluetooth not connecting:**
- Verify Bluetooth name is "ESP32_Parking"
- Check serial monitor for initialization message
- Restart ESP32 and re-pair

### Backend Issues

**Bluetooth connection failed:**
- Verify rfcomm binding: `ls -l /dev/rfcomm0`
- Check pairing: `bluetoothctl devices`
- Verify user permissions: `sudo usermod -a -G dialout $USER`

**OTP not displaying on LCD:**
- Check backend logs for "DisplayOTP" calls
- Verify Bluetooth bridge is connected
- Test serial communication: `sudo screen /dev/rfcomm0 115200`

**Database errors:**
- Ensure SQLite database file exists and is writable
- Check `DBPath` in `.env` configuration

## Testing

### Manual Testing

1. **Test ESP32 locally:**
```bash
sudo screen /dev/rfcomm0 115200
# Type: status
# Type: 1:1234 (to display OTP)
# Type: toggle 1 (to toggle gate)
```

2. **Test API with curl:**
```bash
# Reserve a spot
curl -X POST http://localhost:8080/api/parks/park-1/reserve

# Validate OTP
curl -X POST http://localhost:8080/api/parks/park-1/spots/park-1-spot-1/validate-otp \
  -H "Content-Type: application/json" \
  -d '{"otp":"1234"}'
```

3. **Test occupancy detection:**
- Place object < 40cm from sensor
- Check backend logs for UPDATE messages
- Verify LCD shows occupancy (*)

## Project Structure

```
mdp-ir-car-parking/
├── backend-go/
│   ├── bluetooth/          # Bluetooth bridge and handler
│   │   ├── bluetooth.go    # Serial communication
│   │   └── handler.go      # Message handling
│   ├── config/             # Configuration loading
│   ├── middleware/         # Auth middleware
│   ├── parking/            # Parking logic and OTP system
│   │   └── parking.go      # Core parking + OTP implementation
│   ├── query/              # Database queries
│   ├── routes/             # Auth routes
│   ├── main.go            # Entry point
│   ├── go.mod             # Dependencies
│   └── .env               # Configuration
├── esp32/
│   └── sketch_feb12a_copy_20260212020616/
│       └── *.ino          # ESP32 firmware
├── frontend-app/          # Tauri mobile/desktop app
├── BLUETOOTH_SETUP.md     # Detailed Bluetooth setup guide
└── README.md             # This file
```

## Migration from QR System

Changes made from QR code authentication to OTP system:

1. **Removed QR library**: `boombuler/barcode` package removed
2. **Added OTP fields**: `Spot.OTP` and `Spot.OTPExpiry` replacing `Spot.QRCode`
3. **New endpoint**: `/validate-otp` for OTP validation
4. **ESP32 LCD**: Now displays OTP codes instead of gate-only status
5. **Bluetooth integration**: Direct backend ↔ ESP32 communication
6. **Single-use codes**: QR codes could be reused; OTPs cannot

## Future Enhancements

- [ ] Mobile app UI for OTP input
- [ ] SMS/Email OTP delivery
- [ ] Multi-park support
- [ ] Reservation time limits
- [ ] Usage statistics and analytics
- [ ] Remote gate control via web dashboard
- [ ] Video surveillance integration
- [ ] Payment gateway integration

## License

See [LICENSE](LICENSE) file for details.

## Contributors

- ESP32 firmware development
- Go backend with OTP system
- Bluetooth bridge implementation
- System integration and testing

## Support

For issues and questions:
1. Check [BLUETOOTH_SETUP.md](BLUETOOTH_SETUP.md) for connectivity issues
2. Review backend logs for error messages
3. Test components individually (ESP32, backend, Bluetooth)
4. Verify hardware connections and power supply
