#!/usr/bin/env python3
"""
Bluetooth Bridge for ESP32 Parking System
Connects ESP32 via Bluetooth and bridges to Go backend HTTP API
"""

import bluetooth
import time
import sys
import requests
import threading

ESP32_NAME = "ESP32_Parking"
BACKEND_URL = "http://localhost:8080"

def find_esp32():
    """Search for ESP32_Parking device"""
    print(f"Searching for {ESP32_NAME}...")
    nearby_devices = bluetooth.discover_devices(duration=8, lookup_names=True, lookup_class=False)
    
    for addr, name in nearby_devices:
        print(f"Found: {name} - {addr}")
        if name == ESP32_NAME:
            print(f"✓ Found {ESP32_NAME} at {addr}")
            return addr
    
    return None

def connect_esp32(mac_address):
    """Connect to ESP32 via Bluetooth Serial"""
    print(f"Connecting to {mac_address}...")
    try:
        sock = bluetooth.BluetoothSocket(bluetooth.RFCOMM)
        sock.connect((mac_address, 1))
        print("✓ Connected successfully!")
        return sock
    except Exception as e:
        print(f"✗ Connection failed: {e}")
        return None

def update_backend_occupancy(spot, occupied):
    """Update backend API with occupancy change"""
    try:
        url = f"{BACKEND_URL}/api/parks/park-1/spots/park-1-spot-{spot}/update"
        data = {"occupied": occupied}
        response = requests.post(url, json=data, timeout=2)
        if response.status_code == 200:
            print(f"✓ Updated spot {spot}: {'OCCUPIED' if occupied else 'EMPTY'}")
        else:
            print(f"✗ Backend error: {response.status_code}")
    except Exception as e:
        print(f"✗ Backend error: {e}")

def trigger_gate_open(spot):
    """Trigger gate open on backend API"""
    try:
        url = f"{BACKEND_URL}/api/parks/park-1/spots/park-1-spot-{spot}/opengate"
        response = requests.post(url, json={}, timeout=2)
        if response.status_code == 200:
            print(f"✓ Gate {spot} opened")
    except Exception as e:
        print(f"✗ Backend error: {e}")

def listen_to_esp32(sock):
    """Background thread to listen for ESP32 messages"""
    sock.settimeout(1.0)
    buffer = ""
    
    while True:
        try:
            data = sock.recv(1024)
            if not data:
                break
            
            buffer += data.decode('utf-8', errors='ignore')
            
            # Process complete lines
            while '\n' in buffer:
                line, buffer = buffer.split('\n', 1)
                line = line.strip()
                
                if not line:
                    continue
                
                # Parse messages from ESP32
                if line.startswith("UPDATE:"):
                    # Format: UPDATE:1:1 or UPDATE:2:0
                    parts = line.split(':')
                    if len(parts) == 3:
                        spot = int(parts[1])
                        occupied = parts[2] == '1'
                        update_backend_occupancy(spot, occupied)
                
                elif line.startswith("GATE:"):
                    # Format: GATE:1:OPEN or GATE:1:CLOSED
                    parts = line.split(':')
                    if len(parts) == 3:
                        spot = int(parts[1])
                        state = parts[2]
                        if state == "OPEN":
                            trigger_gate_open(spot)
                        print(f"✓ Gate {spot}: {state}")
                
                elif line.startswith("STATUS:"):
                    # Format: STATUS:E:O:E
                    print(f"Status: {line}")
                
                else:
                    # Other messages
                    print(f"ESP32: {line}")
        
        except bluetooth.btcommon.BluetoothError:
            continue  # Timeout is expected
        except Exception as e:
            print(f"Error: {e}")
            break

def send_command(sock, command):
    """Send command to ESP32"""
    try:
        sock.send(command.encode() + b'\n')
        time.sleep(0.1)
        return True
    except Exception as e:
        print(f"Error sending command: {e}")
        return False

def interactive_mode(sock):
    """Interactive command mode"""
    print("\n=== ESP32 Parking Bridge (Bluetooth ↔ Backend) ===")
    print("Commands:")
    print("  1, 2, 3    - Toggle gate 1, 2, or 3 (open/close)")
    print("  status (s) - Show parking status")
    print("  quit (q)   - Exit")
    print()
    print("Note: Gates toggle on each press - open → closed → open")
    print()
    
    # Start background thread to listen for ESP32 messages
    listener = threading.Thread(target=listen_to_esp32, args=(sock,), daemon=True)
    listener.start()
    
    while True:
        try:
            cmd = input("Command> ").strip().lower()
            
            if cmd in ['q', 'quit', 'exit']:
                print("Disconnecting...")
                break
            elif cmd in ['1', '2', '3']:
                send_command(sock, cmd)
            elif cmd in ['s', 'status']:
                send_command(sock, "status")
            elif cmd == '':
                continue
            else:
                print(f"Unknown command: {cmd}")
        except KeyboardInterrupt:
            print("\nInterrupted. Disconnecting...")
            break
        except Exception as e:
            print(f"Error: {e}")
            break

def main():
    """Main function"""
    print("ESP32 Parking Bluetooth Bridge")
    print("=" * 40)
    
    # Find ESP32
    mac = find_esp32()
    if not mac:
        print(f"\n✗ Could not find {ESP32_NAME}")
        print("\nMake sure:")
        print("  1. ESP32 is powered on")
        print("  2. Bluetooth is enabled on your laptop")
        print("  3. You're not already connected from another device")
        sys.exit(1)
    
    # Connect
    sock = connect_esp32(mac)
    if not sock:
        print("\n✗ Connection failed!")
        sys.exit(1)
    
    print(f"Backend API: {BACKEND_URL}")
    print("Bridge active - forwarding ESP32 events to backend\n")
    
    try:
        # Enter interactive mode
        interactive_mode(sock)
    finally:
        sock.close()
        print("Disconnected.")

if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\nExiting...")
        sys.exit(0)
