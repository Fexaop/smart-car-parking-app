import bluetooth
import time
import sys
import json
import threading
import select
import os
import re
import subprocess

ESP32_NAME = "ESP32_Parking"
DEFAULT_RECONNECT_SECONDS = 3

def log_error(msg):
    print(msg, file=sys.stderr, flush=True)

def log_info(msg):
    print(msg, file=sys.stderr, flush=True)

def send_to_go(message_type, data):
    msg = json.dumps({"type": message_type, "data": data})
    print(msg, flush=True)

def get_reconnect_delay_seconds():
    value = os.getenv("ESP32_RECONNECT_SECONDS", str(DEFAULT_RECONNECT_SECONDS)).strip()
    try:
        parsed = int(value)
        if parsed < 1:
            return DEFAULT_RECONNECT_SECONDS
        return parsed
    except ValueError:
        return DEFAULT_RECONNECT_SECONDS

def get_configured_mac():
    for key in ("ESP32_MAC", "ESP32_BT_MAC"):
        value = os.getenv(key, "").strip()
        if not value:
            continue
        if re.match(r"^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$", value):
            return value.upper()
        log_error(f"Ignoring invalid MAC in {key}: {value}")
    return None

def find_from_paired_devices():
    try:
        result = subprocess.run(
            ["bluetoothctl", "devices"],
            capture_output=True,
            text=True,
            timeout=5,
            check=False,
        )
    except Exception as e:
        log_error(f"Could not query paired devices: {e}")
        return None

    for raw in result.stdout.splitlines():
        line = raw.strip()
        if not line.startswith("Device "):
            continue
        parts = line.split(maxsplit=2)
        if len(parts) < 3:
            continue
        address = parts[1].strip().upper()
        name = parts[2].strip()
        if name == ESP32_NAME:
            log_info(f"Found paired {ESP32_NAME} at {address}")
            return address
    return None

def discover_rfcomm_channels(mac_address):
    channels = []
    try:
        services = bluetooth.find_service(address=mac_address)
        for service in services:
            if service.get("protocol") == "RFCOMM":
                port = service.get("port")
                if isinstance(port, int) and port > 0 and port not in channels:
                    channels.append(port)
    except Exception as e:
        log_error(f"RFCOMM service lookup failed: {e}")

    if 1 not in channels:
        channels.append(1)
    return channels

def find_esp32():
    configured_mac = get_configured_mac()
    if configured_mac:
        log_info(f"Using configured ESP32 MAC: {configured_mac}")
        return configured_mac

    paired_mac = find_from_paired_devices()
    if paired_mac:
        return paired_mac

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
    log_info(f"Connecting to {mac_address}...")
    channels = discover_rfcomm_channels(mac_address)

    for channel in channels:
        try:
            sock = bluetooth.BluetoothSocket(bluetooth.RFCOMM)
            sock.connect((mac_address, channel))
            log_info(f"Connected successfully on RFCOMM channel {channel}")
            send_to_go("connected", {"address": mac_address, "channel": channel})
            return sock
        except Exception as e:
            log_error(f"Connection attempt failed on channel {channel}: {e}")

    send_to_go("error", {"message": f"Connection failed for {mac_address}"})
    return None

def listen_to_esp32(sock, stop_event):
    sock.settimeout(1.0)
    buffer = ""
    
    while not stop_event.is_set():
        try:
            data = sock.recv(1024)
            if not data:
                log_error("ESP32 disconnected")
                send_to_go("disconnected", {"reason": "socket closed"})
                stop_event.set()
                break
            
            buffer += data.decode('utf-8', errors='ignore')
            
            while '\n' in buffer:
                line, buffer = buffer.split('\n', 1)
                line = line.strip()
                
                if not line:
                    continue
                
                log_info(f"ESP32: {line}")
                if line.startswith("UPDATE:"):
                    parts = line.split(':')
                    if len(parts) == 3:
                        spot = int(parts[1])
                        occupied = parts[2] == '1'
                        send_to_go("update", {"spot": spot, "occupied": occupied})
                
                elif line.startswith("GATE:"):
                    parts = line.split(':')
                    if len(parts) == 3:
                        spot = int(parts[1])
                        state = parts[2]
                        send_to_go("gate", {"spot": spot, "state": state})
                
                elif line.startswith("STATUS:"):
                    send_to_go("status", {"message": line})
                
                elif line.startswith("OTP_VALID:"):
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
                    send_to_go("message", {"text": line})
        
        except bluetooth.btcommon.BluetoothError as e:
            message = str(e).lower()
            if "timed out" in message:
                continue
            log_error(f"Bluetooth error in listener: {e}")
            send_to_go("disconnected", {"reason": str(e)})
            stop_event.set()
            break
        except Exception as e:
            log_error(f"Error in listener: {e}")
            send_to_go("error", {"message": str(e)})
            send_to_go("disconnected", {"reason": str(e)})
            stop_event.set()
            break

def send_command(sock, command):
    try:
        sock.send(command.encode() + b'\n')
        log_info(f"Sent to ESP32: {command}")
        return True
    except Exception as e:
        log_error(f"Error sending command: {e}")
        send_to_go("error", {"message": f"Send failed: {e}"})
        return False

def read_stdin_commands(sock, stop_event):
    log_info("Stdin command reader started")
    
    while not stop_event.is_set():
        try:
            if select.select([sys.stdin], [], [], 1.0)[0]:
                line = sys.stdin.readline()
                if not line:
                    log_info("Stdin closed, exiting")
                    return True
                
                command = line.strip()
                if command:
                    log_info(f"Received command from Go: {command}")
                    if not send_command(sock, command):
                        stop_event.set()
                        return False
            else:
                continue
                
        except Exception as e:
            log_error(f"Error reading stdin: {e}")
            stop_event.set()
            return False

    return False

def reconnect_notice(delay_seconds):
    log_info(f"Reconnecting to ESP32 in {delay_seconds}s...")
    time.sleep(delay_seconds)

def main():
    """Main function"""
    log_info("ESP32 Parking Bluetooth Bridge (Subprocess Mode)")
    log_info("=" * 50)
    reconnect_delay_seconds = get_reconnect_delay_seconds()

    while True:
        mac = find_esp32()
        if not mac:
            log_error(f"Could not find {ESP32_NAME}")
            send_to_go("error", {"message": f"Device {ESP32_NAME} not found"})
            reconnect_notice(reconnect_delay_seconds)
            continue

        sock = connect_esp32(mac)
        if not sock:
            reconnect_notice(reconnect_delay_seconds)
            continue

        log_info("Bridge active - ready for commands")
        send_to_go("ready", {})

        stop_event = threading.Event()
        listener = threading.Thread(target=listen_to_esp32, args=(sock, stop_event), daemon=True)
        listener.start()

        try:
            stdin_closed = read_stdin_commands(sock, stop_event)
        except KeyboardInterrupt:
            stdin_closed = True
            log_info("Interrupted")

        stop_event.set()
        try:
            sock.close()
        except Exception:
            pass

        if stdin_closed:
            log_info("Bridge shutting down")
            send_to_go("shutdown", {})
            return

        send_to_go("disconnected", {"reason": "reconnect"})
        reconnect_notice(reconnect_delay_seconds)

if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        log_error(f"Fatal error: {e}")
        send_to_go("error", {"message": f"Fatal: {e}"})
        sys.exit(1)
