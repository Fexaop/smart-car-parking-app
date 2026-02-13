package bluetooth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type ParkingMessageHandler struct {
	backendURL string
}

func NewParkingMessageHandler(backendURL string) *ParkingMessageHandler {
	return &ParkingMessageHandler{
		backendURL: backendURL,
	}
}

func (h *ParkingMessageHandler) HandleUpdate(spot int, occupied bool) {
	spotID := fmt.Sprintf("park-1-spot-%d", spot)
	url := fmt.Sprintf("%s/api/parks/park-1/spots/%s/update", h.backendURL, spotID)
	
	payload := map[string]bool{"occupied": occupied}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal update payload: %v\n", err)
		return
	}
	
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to update backend: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	log.Printf("Updated spot %d occupancy: %v (status: %d)\n", spot, occupied, resp.StatusCode)
}

func (h *ParkingMessageHandler) HandleGateState(spot int, state string) {
	log.Printf("Gate %d state: %s\n", spot, state)
	// Could be used for monitoring/logging
}

func (h *ParkingMessageHandler) HandleStatus(status string) {
	log.Printf("ESP32 Status: %s\n", status)
}
