# Quick Start Guide: OTP Parking System

## Prerequisites Check

Before starting, verify you have:
- ✅ ESP32 uploaded with latest firmware
- ✅ Go 1.21+ installed
- ✅ Python 3.7+ installed
- ✅ Linux/macOS system with Bluetooth
- ✅ pybluez library: `pip3 install pybluez`

## Step-by-Step Setup (5 Minutes)

### Step 1: Install Python Dependencies

```bash
# Linux
sudo apt-get install bluetooth libbluetooth-dev python3-dev
pip3 install pybluez

# macOS
brew install pybluez
pip3 install pybluez

# Verify
python3 -c "import bluetooth; print('pybluez installed successfully')"
```

### Step 2: Pair ESP32 via Bluetooth

```bash
# Start Bluetooth service (Linux)
sudo systemctl start bluetooth

# Open Bluetooth control
bluetoothctl

# In bluetoothctl prompt:
power on
scan on
# Wait for "ESP32_Parking" to appear, note its MAC address
# Example: [NEW] Device AA:BB:CC:DD:EE:FF ESP32_Parking

pair AA:BB:CC:DD:EE:FF
trust AA:BB:CC:DD:EE:FF
exit
```

**Note:** You don't need to manually connect or create rfcomm bindings. The Python bridge will handle the connection automatically.

### Step 3: Start Backend Server

The Go backend automatically spawns the Python Bluetooth bridge as a subprocess.

```bash
cd backend-go

# The backend is already built, just run it
./mdp-ir-car-parking

# You should see:
# Starting Python Bluetooth bridge: ../bluetooth_bridge.py
# [Python] Searching for ESP32_Parking...
# [Python] Found: ESP32_Parking - AA:BB:CC:DD:EE:FF
# [Python] ✓ Connected successfully!
# Python bridge ready
# Server running on http://localhost:8080
```

**If you see errors:** Check that pybluez is installed and ESP32 is paired.

## Testing the System

### Test 1: Check ESP32 Status

Open a new terminal:
```bash
curl http://localhost:8080/api/parks
```

You should see JSON with 3 parking spots.

### Test 2: Reserve a Spot (Generate OTP)

```bash
curl -X POST http://localhost:8080/api/parks/park-1/reserve
```

**Expected Response:**
```json
{
  "parkId": "park-1",
  "spotId": "park-1-spot-1",
  "spotNumber": 1,
  "otp": "1234",
  "otpExpiry": "2024-02-12T15:30:00Z"
}
```

**Check ESP32 LCD - You should see:**
```
Spot 1 OTP:
Code: 1234
```
(This will display for 30 seconds, then return to normal status)

### Test 3: Validate OTP and Open Gate

```bash
curl -X POST http://localhost:8080/api/parks/park-1/spots/park-1-spot-1/validate-otp \
  -H "Content-Type: application/json" \
  -d '{"otp":"1234"}'
```

**Expected Response:**
```json
{
  "ok": true,
  "message": "Gate opened successfully"
}
```

**Gate 1 should physically open!** (Servo rotates to 90°)

### Test 4: Manual Gate Toggle

Press the physical button on pin 32 to toggle gate 1 closed.

Or use Bluetooth command:
```bash
# Open serial terminal
sudo screen /dev/rfcomm0 115200

# Type commands:
toggle 1    # Toggle gate 1
status      # Get status
# Press Ctrl+A then K to exit screen
```

### Test 5: Occupancy Detection

Place an object less than 40cm in front of ultrasonic sensor 1.

**Check backend logs - You should see:**
```
Updated spot 1 occupancy: true (status: 200)
```

**Check LCD - Should show:**
```
G1:* G2:C G3:C
15cm 45cm 50cm
```
(The * indicates occupied)

## Troubleshooting

### Issue: "Device ESP32_Parking not found"

**Solution:**
```bash
# Check if ESP32 is paired
bluetoothctl devices | grep ESP32

# If not paired, pair it:
bluetoothctl
> power on
> scan on
# Wait for ESP32_Parking
> pair <MAC_ADDRESS>
> trust <MAC_ADDRESS>
> exit

# Restart backend
cd backend-go
./mdp-ir-car-parking
```

### Issue: "Failed to start Bluetooth bridge"

**Check Python dependencies:**
```bash
# Verify Python 3
python3 --version

# Verify pybluez
python3 -c "import bluetooth; print('OK')"

# If not installed:
pip3 install pybluez

# On Linux, you may also need:
sudo apt-get install bluetooth libbluetooth-dev python3-dev
```

### Issue: "Connection failed" (Python)

**Solution:**
```bash
# Make sure ESP32 is not connected to another device
bluetoothctl
> info <ESP32_MAC>
# Check if "Connected: yes" to another device

# Disconnect from other devices:
> disconnect <ESP32_MAC>
> exit

# Restart ESP32 physically (power cycle)

# Restart backend
./mdp-ir-car-parking
```

### Issue: OTP not displaying on LCD

**Check:**
1. Backend logs show "[Python] Sent to ESP32: 1:1234"
2. ESP32 serial monitor shows OTP received
3. Python bridge is connected (logs show "Python bridge ready")

**Test manually:**
```bash
# Test Python bridge directly
python3 ../bluetooth_bridge.py
# In another terminal, simulating Go:
echo "1:9999" > /tmp/test_cmd
# Watch Python output
```

### Issue: Gate not opening on OTP validation

**Check:**
1. Gate is not already open (check LCD: G1:O means already open)
2. Backend sends "OPEN:1" command (check logs)
3. Python bridge forwards command to ESP32
4. Servo is connected to correct pin (25, 26, or 27)

**Test manually:**
```bash
# If Python bridge is running standalone:
echo "OPEN:1" | python3 -c "
import sys
sys.stdout.write(sys.stdin.read())
" | python3 ../bluetooth_bridge.py
```

### Issue: Backend can't connect to ESP32

**Check:**
1. ESP32 is powered on and Bluetooth is initialized
2. Serial monitor shows "BT: ESP32_Parking"
3. ESP32 is not connected to another device
4. rfcomm binding exists: `ls -l /dev/rfcomm0`

**Reset and retry:**
```bash
# Release binding
sudo rfcomm release /dev/rfcomm0

# Disconnect and reconnect ESP32
bluetoothctl
> disconnect <ESP32_MAC>
> connect <ESP32_MAC>
> exit

# Rebind
sudo rfcomm bind /dev/rfcomm0 <ESP32_MAC> 1

# Restart backend
./mdp-ir-car-parking
```

## System Status Indicators

### Backend Logs (Healthy)
```
Starting Python Bluetooth bridge: ../bluetooth_bridge.py
[Python] Searching for ESP32_Parking...
[Python] Found: ESP32_Parking - AA:BB:CC:DD:EE:FF
[Python] ✓ Connected successfully!
Python bridge ready
Server running on http://localhost:8080
```

### Backend Logs (Unhealthy)
```
Failed to start Bluetooth bridge: pybluez not installed
Parking system will run without Bluetooth integration
```
**Fix:** Install pybluez and restart

### ESP32 Serial Monitor (Healthy)
```
BT: ESP32_Parking
Gate 1 OPEN
OTP for spot 1: 1234
```

### ESP32 LCD (Normal Operation)
```
G1:O G2:C G3:C
45cm 38cm 50cm
```

### ESP32 LCD (OTP Display)
```
Spot 1 OTP:
Code: 1234
```

## Complete Test Sequence

Run this full flow to verify everything works:

```bash
# 1. Start backend
cd backend-go
./mdp-ir-car-parking

# 2. In new terminal - List spots
curl http://localhost:8080/api/parks

# 3. Reserve spot 1
curl -X POST http://localhost:8080/api/parks/park-1/reserve | jq .

# 4. Check LCD - should show OTP for 30 seconds

# 5. Copy OTP from response, then validate it
curl -X POST http://localhost:8080/api/parks/park-1/spots/park-1-spot-1/validate-otp \
  -H "Content-Type: application/json" \
  -d '{"otp":"XXXX"}' | jq .
# Replace XXXX with actual OTP from step 3

# 6. Gate 1 should open

# 7. Try to reuse OTP (should fail)
curl -X POST http://localhost:8080/api/parks/park-1/spots/park-1-spot-1/validate-otp \
  -H "Content-Type: application/json" \
  -d '{"otp":"XXXX"}' | jq .
# Should return error: "no active reservation"

# 8. Toggle gate closed manually via button or:
sudo screen /dev/rfcomm0 115200
# Type: toggle 1

# 9. Release spot
curl -X POST http://localhost:8080/api/parks/park-1/spots/park-1-spot-1/release

# SUCCESS! ✅
```

## What's Next?

Now that the system is working:

1. **Mobile App Integration**
   - Update frontend app to use OTP instead of QR codes
   - Add numeric OTP input field
   - Call `/validate-otp` endpoint

2. **Make Bluetooth Permanent**
   - Set up systemd service for rfcomm (see BLUETOOTH_SETUP.md)
   - Set up systemd service for backend

3. **Production Deployment**
   - Configure logging
   - Set up monitoring
   - Add HTTPS/SSL
   - Configure firewall rules

## Need More Help?

- **Bluetooth Setup**: See [BLUETOOTH_SETUP.md](BLUETOOTH_SETUP.md)
- **Full Documentation**: See [OTP_README.md](OTP_README.md)
- **Migration Details**: See [MIGRATION_SUMMARY.md](MIGRATION_SUMMARY.md)

## Support

If you're still having issues:
1. Check all physical connections (sensors, servos, buttons, LCD)
2. Verify power supply is adequate (USB power may not be enough for all servos)
3. Review backend logs for specific error messages
4. Test each component individually (sensors, servos, Bluetooth, LCD)

Good luck! 🚀
