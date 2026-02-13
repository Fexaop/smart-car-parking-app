package bluetooth

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
)

type BluetoothBridge struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	mu      sync.Mutex
	handler MessageHandler
	running bool
}

type MessageHandler interface {
	HandleUpdate(spot int, occupied bool)
	HandleGateState(spot int, state string)
	HandleStatus(status string)
}

type PythonMessage struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

func NewBluetoothBridge(handler MessageHandler) *BluetoothBridge {
	return &BluetoothBridge{
		handler: handler,
	}
}

// StartPythonBridge starts the Python Bluetooth bridge as a subprocess
func (b *BluetoothBridge) StartPythonBridge(pythonScript string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return fmt.Errorf("bridge already running")
	}

	// Start Python subprocess
	b.cmd = exec.Command("python3", pythonScript)
	
	var err error
	b.stdin, err = b.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	
	b.stdout, err = b.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	
	b.stderr, err = b.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	if err := b.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Python bridge: %w", err)
	}
	
	b.running = true
	log.Println("Python Bluetooth bridge started")
	
	// Start reading from stdout (structured messages)
	go b.readStdout()
	
	// Start reading from stderr (logs)
	go b.readStderr()
	
	return nil
}

func (b *BluetoothBridge) readStdout() {
	scanner := bufio.NewScanner(b.stdout)
	
	for scanner.Scan() {
		line := scanner.Text()
		
		var msg PythonMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			log.Printf("Failed to parse Python message: %s (error: %v)\n", line, err)
			continue
		}
		
		b.handlePythonMessage(msg)
	}
	
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from Python stdout: %v\n", err)
	}
	
	log.Println("Python stdout reader stopped")
}

func (b *BluetoothBridge) readStderr() {
	scanner := bufio.NewScanner(b.stderr)
	
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("[Python] %s\n", line)
	}
	
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from Python stderr: %v\n", err)
	}
}

func (b *BluetoothBridge) handlePythonMessage(msg PythonMessage) {
	switch msg.Type {
	case "connected":
		log.Printf("ESP32 connected: %v\n", msg.Data["address"])
	
	case "ready":
		log.Println("Python bridge ready")
	
	case "update":
		if b.handler != nil {
			spot := int(msg.Data["spot"].(float64))
			occupied := msg.Data["occupied"].(bool)
			b.handler.HandleUpdate(spot, occupied)
		}
	
	case "gate":
		if b.handler != nil {
			spot := int(msg.Data["spot"].(float64))
			state := msg.Data["state"].(string)
			b.handler.HandleGateState(spot, state)
		}
	
	case "status":
		if b.handler != nil {
			status := msg.Data["message"].(string)
			b.handler.HandleStatus(status)
		}
	
	case "error":
		log.Printf("Python bridge error: %v\n", msg.Data["message"])
	
	case "disconnected":
		log.Println("ESP32 disconnected")
	
	case "shutdown":
		log.Println("Python bridge shutdown")
	
	case "message":
		log.Printf("ESP32 message: %v\n", msg.Data["text"])
	
	default:
		log.Printf("Unknown message type: %s\n", msg.Type)
	}
}

func (b *BluetoothBridge) SendCommand(cmd string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if !b.running || b.stdin == nil {
		return fmt.Errorf("bridge not running")
	}
	
	_, err := b.stdin.Write([]byte(cmd + "\n"))
	if err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}
	
	log.Printf("Sent to Python bridge: %s\n", cmd)
	return nil
}

func (b *BluetoothBridge) DisplayOTP(otp string) error {
	return b.SendCommand(otp)
}

func (b *BluetoothBridge) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if !b.running {
		return nil
	}
	
	// Close stdin to signal shutdown
	if b.stdin != nil {
		b.stdin.Close()
	}
	
	// Wait for process to exit
	if b.cmd != nil && b.cmd.Process != nil {
		b.cmd.Wait()
	}
	
	b.running = false
	log.Println("Python bridge stopped")
	
	return nil
}
