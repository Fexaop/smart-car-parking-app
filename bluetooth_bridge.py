#!/usr/bin/env python3
"""
Bluetooth Bridge Subprocess for Go Backend
Connects ESP32 via Bluetooth and communicates with Go via stdin/stdout
"""

import bluetooth
import time
import sys
import json
import threading
import select

ESP32_NAME = "ESP32_Parking"

def log_error(msg):
    """Log to stderr (Go can read this)"""
    print(msg, file=sys.stderr, flush=True)

def log_info(msg):
    """Log to stderr (Go can read this)"""
    print(msg, file=sys.stderr, flush=True)

def send_to_go(message_type, data):
    """Send structured message to Go via stdout"""
    msg = json.dumps({"type": message_type, "data": data})
    print(msg, flush=True)

def find_esp32():
    """Search for ESP32_Parking device"""
    log_info(f"Searching for {ESP32_NAME}...")
    try:
        nearby_devices = bluetooth.discover_devices(duration=8, lookup_names=True, lookup_class=False)
        
        for addr, name in nearby_devices:
            log_info(f"Found: {name} - {addr}")
            if name == ESP32_NAME:
                log_info(f"✓ Found {ESP32_NAME} at {addr}")
                return addr
    except Exception as e:
        log_error(f"Error during discovery: {e}")
    
    return None

def connect_esp32(mac_address):
    """Connect to ESP32 via Bluetooth Serial"""
    log_info(f"Connecting to {mac_address}...")
    try:
        sock = bluetooth.BluetoothSocket(bluetooth.RFCOMM)
        sock.connect((mac_address, 1))
        log_info("✓ Connected successfully!")
        send_to_go("connected", {"address": mac_address})
        return sock
    except Exception as e:
        log_error(f"✗ Connection failed: {e}")
        send_to_go("error", {"message": f"Connection failed: {e}"})
        return None

def listen_to_esp32(sock):
    """Background thread to listen for ESP32 messages and forward to Go"""
    sock.settimeout(1.0)
    buffer = ""
    
    while True:
        try:
            data = sock.recv(1024)
            if not data:
                log_error("ESP32 disconnected")
                send_to_go("disconnected", {})
                break
            
            buffer += data.decode('utf-8', errors='ignore')
            
            # Process complete lines
            while '\n' in buffer:
                line, buffer = buffer.split('\n', 1)
                line = line.strip()
                
                if not line:
                    continue
                
                log_info(f"ESP32: {line}")
                
                # Parse messages from ESP32
                if line.startswith("UPDATE:"):
                    # Format: UPDATE:1:1 or UPDATE:2:0
                    parts = line.split(':')
                    if len(parts) == 3:
                        spot = int(parts[1])
                        occupied = parts[2] == '1'
                        send_to_go("update", {"spot": spot, "occupied": occupied})
                
                elif line.startswith("GATE:"):
                    # Format: GATE:1:OPEN or GATE:1:CLOSED
                    parts = line.split(':')
                    if len(parts) == 3:
                        spot = int(parts[1])
                        state = parts[2]
                        send_to_go("gate", {"spot": spot, "state": state})
                
                elif line.startswith("STATUS:"):
                    # Format: STATUS:E:O:E
                    send_to_go("status", {"message": line})
                
                elif line.startswith("OTP_VALID:"):
                    # Format: OTP_VALID:1
                    parts = line.split(':')
                    if len(parts) == 2:
                        spot = int(parts[1])
                        send_to_go("otp_valid", {"spot": spot})
                        log_info(f"OTP validated successfully for spot {spot}")
                
                elif line == "OTP_INVALID":
                    send_to_go("otp_invalid", {})
                    log_info("OTP validation failed")
                
                elif line == "OTP_CLEARED":
                    send_to_go("otp_cleared", {})
                    log_info("OTP display cleared")
                
                else:
                    # Other messages
                    send_to_go("message", {"text": line})
        
        except bluetooth.btcommon.BluetoothError:
            continue  # Timeout is expected
        except Exception as e:
            log_error(f"Error in listener: {e}")
            send_to_go("error", {"message": str(e)})
            break

def send_command(sock, command):
    """Send command to ESP32"""
    try:
        sock.send(command.encode() + b'\n')
        log_info(f"Sent to ESP32: {command}")
        return True
    except Exception as e:
        log_error(f"Error sending command: {e}")
        send_to_go("error", {"message": f"Send failed: {e}"})
        return False

def read_stdin_commands(sock):
    """Read commands from stdin (sent by Go) and forward to ESP32"""
    log_info("Stdin command reader started")
    
    while True:
        try:
            # Use select to check if stdin has data (non-blocking)
            if select.select([sys.stdin], [], [], 1.0)[0]:
                line = sys.stdin.readline()
                if not line:
                    # EOF - Go process closed stdin
                    log_info("Stdin closed, exiting")
                    break
                
                command = line.strip()
                if command:
                    log_info(f"Received command from Go: {command}")
                    send_command(sock, command)
            else:
                # No data available, continue
                continue
                
        except Exception as e:
            log_error(f"Error reading stdin: {e}")
            break

def main():
    """Main function"""
    log_info("ESP32 Parking Bluetooth Bridge (Subprocess Mode)")
    log_info("=" * 50)
    
    # Find ESP32
    mac = find_esp32()
    if not mac:
        log_error(f"Could not find {ESP32_NAME}")
        send_to_go("error", {"message": f"Device {ESP32_NAME} not found"})
        sys.exit(1)
    
    # Connect
    sock = connect_esp32(mac)
    if not sock:
        log_error("Connection failed!")
        send_to_go("error", {"message": "Connection failed"})
        sys.exit(1)
    
    log_info("Bridge active - ready for commands")
    send_to_go("ready", {})
    
    # Start background thread to listen for ESP32 messages
    listener = threading.Thread(target=listen_to_esp32, args=(sock,), daemon=True)
    listener.start()
    
    # Start reading commands from stdin (Go will send commands here)
    try:
        read_stdin_commands(sock)
    except KeyboardInterrupt:
        log_info("Interrupted")
    finally:
        sock.close()
        log_info("Disconnected")
        send_to_go("shutdown", {})

if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        log_error(f"Fatal error: {e}")
        send_to_go("error", {"message": f"Fatal: {e}"})
        sys.exit(1)
