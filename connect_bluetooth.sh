#!/bin/bash
# Bluetooth Connection Script for ESP32 Parking System
# For Linux systems using rfcomm

ESP32_NAME="ESP32_Parking"

echo "ESP32 Parking Bluetooth Controller"
echo "===================================="
echo ""

# Check if bluetoothctl is available
if ! command -v bluetoothctl &> /dev/null; then
    echo "Error: bluetoothctl not found. Install bluez package."
    exit 1
fi

# Scan for ESP32
echo "Scanning for $ESP32_NAME..."
timeout 10 bluetoothctl scan on > /dev/null 2>&1 &
SCAN_PID=$!
sleep 8
kill $SCAN_PID 2>/dev/null

# Get MAC address
MAC=$(bluetoothctl devices | grep "$ESP32_NAME" | awk '{print $2}')

if [ -z "$MAC" ]; then
    echo "Error: $ESP32_NAME not found"
    echo ""
    echo "Make sure:"
    echo "  1. ESP32 is powered on"
    echo "  2. Bluetooth is enabled: sudo systemctl start bluetooth"
    echo "  3. You're not already connected from another device"
    exit 1
fi

echo "Found $ESP32_NAME at $MAC"
echo ""

# Pair if not already paired
echo "Pairing..."
bluetoothctl pair $MAC 2>/dev/null
bluetoothctl trust $MAC 2>/dev/null
echo "Connecting..."
bluetoothctl connect $MAC

if [ $? -ne 0 ]; then
    echo "Connection failed. Trying alternate method..."
    # Try using rfcomm
    if command -v rfcomm &> /dev/null; then
        sudo rfcomm bind 0 $MAC 1
        echo "Connected via rfcomm0"
        echo ""
        echo "You can now use: screen /dev/rfcomm0 115200"
        echo "Or: python3 bluetooth_controller.py"
    else
        echo "Error: Could not establish connection"
        exit 1
    fi
else
    echo "Connected successfully!"
    echo ""
    echo "Options:"
    echo "  1. Use the Python controller: python3 bluetooth_controller.py"
    echo "  2. Use a Bluetooth terminal app"
    echo "  3. Use: screen /dev/rfcomm0 115200 (if rfcomm is set up)"
fi
