package parking

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type Spot struct {
    ID       string `json:"id"`
    Number   int    `json:"number"`
    Occupied bool   `json:"occupied"`
    LastOpen string `json:"lastOpen,omitempty"`
    OTP      string `json:"otp,omitempty"`
    OTPExpiry time.Time `json:"otpExpiry,omitempty"`
}

type Park struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Spots []Spot `json:"spots"`
}

type ParkingStore struct {
    mu sync.Mutex
    parks map[string]*Park
    btBridge BluetoothBridgeInterface
}

type BluetoothBridgeInterface interface {
    DisplayOTP(otp string) error
}

var store *ParkingStore

// SetBluetoothBridge sets the Bluetooth bridge for OTP display
func SetBluetoothBridge(bridge BluetoothBridgeInterface) {
    store.mu.Lock()
    defer store.mu.Unlock()
    store.btBridge = bridge
}

// generateOTP creates a random 4-digit OTP
func generateOTP() string {
    return fmt.Sprintf("%04d", rand.Intn(10000))
}

func init() {
    rand.Seed(time.Now().UnixNano())
    
    store = &ParkingStore{
        parks: make(map[string]*Park),
    }

    // initialize 1 parking site with 3 spots (matching ESP32 hardware)
    pid := "park-1"
    p := &Park{
        ID: pid,
        Name: "Parking Site 1",
        Spots: make([]Spot, 3),
    }
    for s := 0; s < 3; s++ {
        p.Spots[s] = Spot{
            ID: fmt.Sprintf("%s-spot-%d", pid, s+1),
            Number: s + 1,
            Occupied: false,
        }
    }
    store.parks[pid] = p
}

// RegisterRoutes registers HTTP handlers under /api/parks
func RegisterRoutes(r *mux.Router) {
    r.HandleFunc("/api/parks", listParksHandler).Methods("GET")
    r.HandleFunc("/api/parks/{parkID}/reserve", reserveSpotHandler).Methods("POST")
    r.HandleFunc("/api/parks/{parkID}/spots/{spotID}/reserve", reserveSpecificSpotHandler).Methods("POST")
    r.HandleFunc("/api/parks/{parkID}/spots/{spotID}/release", releaseSpotHandler).Methods("POST")
    r.HandleFunc("/api/parks/{parkID}/spots/{spotID}/update", updateSpotHandler).Methods("POST")
    r.HandleFunc("/api/parks/{parkID}/spots/{spotID}/opengate", openGateHandler).Methods("POST")
    r.HandleFunc("/api/parks/{parkID}/spots/{spotID}/validate-otp", validateOTPHandler).Methods("POST")
    r.HandleFunc("/api/parks/{parkID}/spots/{spotID}/clear-otp-display", clearOTPDisplayHandler).Methods("POST")
    r.HandleFunc("/api/parks/{parkID}/spots/{spotID}/validate-otp-esp32", validateOTPViaESP32Handler).Methods("POST")
}

func listParksHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    store.mu.Lock()
    defer store.mu.Unlock()
    parks := make([]*Park, 0, len(store.parks))
    for _, p := range store.parks {
        parks = append(parks, p)
    }
    json.NewEncoder(w).Encode(parks)
}

type reserveResp struct {
    ParkID string `json:"parkId"`
    SpotID string `json:"spotId"`
    SpotNumber int `json:"spotNumber"`
    OTP string `json:"otp"`
    OTPExpiry string `json:"otpExpiry"`
}

func reserveSpotHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    parkID := vars["parkID"]

    store.mu.Lock()
    defer store.mu.Unlock()
    p, ok := store.parks[parkID]
    if !ok {
        http.Error(w, "park not found", http.StatusNotFound)
        return
    }

    // find first free spot
    var chosen *Spot
    for i := range p.Spots {
        if !p.Spots[i].Occupied {
            p.Spots[i].Occupied = true
            chosen = &p.Spots[i]
            break
        }
    }

    if chosen == nil {
        http.Error(w, "no free spots", http.StatusConflict)
        return
    }

    // Generate OTP valid for 15 minutes
    otp := generateOTP()
    expiry := time.Now().Add(15 * time.Minute)
    chosen.OTP = otp
    chosen.OTPExpiry = expiry

    // Display OTP on ESP32 LCD via Bluetooth
    if store.btBridge != nil {
        store.btBridge.DisplayOTP(fmt.Sprintf("%d:%s", chosen.Number, otp))
    }

    resp := reserveResp{
        ParkID: parkID,
        SpotID: chosen.ID,
        SpotNumber: chosen.Number,
        OTP: otp,
        OTPExpiry: expiry.Format(time.RFC3339),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func reserveSpecificSpotHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    parkID := vars["parkID"]
    spotID := vars["spotID"]

    store.mu.Lock()
    defer store.mu.Unlock()
    p, ok := store.parks[parkID]
    if !ok {
        http.Error(w, "park not found", http.StatusNotFound)
        return
    }

    var chosen *Spot
    for i := range p.Spots {
        if p.Spots[i].ID == spotID {
            chosen = &p.Spots[i]
            break
        }
    }
    if chosen == nil {
        http.Error(w, "spot not found", http.StatusNotFound)
        return
    }
    if chosen.Occupied {
        http.Error(w, "spot already occupied", http.StatusConflict)
        return
    }

    chosen.Occupied = true

    // Generate OTP valid for 15 minutes
    otp := generateOTP()
    expiry := time.Now().Add(15 * time.Minute)
    chosen.OTP = otp
    chosen.OTPExpiry = expiry

    // Display OTP on ESP32 LCD via Bluetooth
    if store.btBridge != nil {
        store.btBridge.DisplayOTP(fmt.Sprintf("%d:%s", chosen.Number, otp))
    }

    resp := reserveResp{
        ParkID: parkID,
        SpotID: chosen.ID,
        SpotNumber: chosen.Number,
        OTP: otp,
        OTPExpiry: expiry.Format(time.RFC3339),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func releaseSpotHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    parkID := vars["parkID"]
    spotID := vars["spotID"]

    store.mu.Lock()
    defer store.mu.Unlock()
    p, ok := store.parks[parkID]
    if !ok {
        http.Error(w, "park not found", http.StatusNotFound)
        return
    }

    for i := range p.Spots {
        if p.Spots[i].ID == spotID {
            p.Spots[i].Occupied = false
            p.Spots[i].OTP = ""
            p.Spots[i].OTPExpiry = time.Time{}
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]any{"ok": true})
            return
        }
    }

    http.Error(w, "spot not found", http.StatusNotFound)
}

// updateSpotHandler accepts POST { "occupied": true/false } to update spot occupancy
func updateSpotHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    parkID := vars["parkID"]
    spotID := vars["spotID"]

    type payload struct {
        Occupied bool `json:"occupied"`
    }

    var pld payload
    if err := json.NewDecoder(r.Body).Decode(&pld); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }

    store.mu.Lock()
    defer store.mu.Unlock()
    p, ok := store.parks[parkID]
    if !ok {
        http.Error(w, "park not found", http.StatusNotFound)
        return
    }

    for i := range p.Spots {
        if p.Spots[i].ID == spotID {
            p.Spots[i].Occupied = pld.Occupied
            if !pld.Occupied {
                p.Spots[i].OTP = ""
                p.Spots[i].OTPExpiry = time.Time{}
            }
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]any{"ok": true})
            return
        }
    }

    http.Error(w, "spot not found", http.StatusNotFound)
}

// openGateHandler toggles the gate for a spot via ESP32
func openGateHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    parkID := vars["parkID"]
    spotID := vars["spotID"]

    store.mu.Lock()
    defer store.mu.Unlock()
    p, ok := store.parks[parkID]
    if !ok {
        http.Error(w, "park not found", http.StatusNotFound)
        return
    }

    for i := range p.Spots {
        if p.Spots[i].ID == spotID {
            p.Spots[i].LastOpen = time.Now().UTC().Format(time.RFC3339)
            
            // Send toggle command to ESP32
            if store.btBridge != nil {
                cmd := fmt.Sprintf("toggle %d", p.Spots[i].Number)
                if err := store.btBridge.DisplayOTP(cmd); err != nil {
                    http.Error(w, fmt.Sprintf("failed to toggle gate: %v", err), http.StatusInternalServerError)
                    return
                }
            }
            
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]any{"ok": true, "message": "Gate toggled"})
            return
        }
    }

    http.Error(w, "spot not found", http.StatusNotFound)
}

// validateOTPHandler validates OTP and opens gate if valid
func validateOTPHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    parkID := vars["parkID"]
    spotID := vars["spotID"]

    type payload struct {
        OTP string `json:"otp"`
    }

    var pld payload
    if err := json.NewDecoder(r.Body).Decode(&pld); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }

    store.mu.Lock()
    defer store.mu.Unlock()
    p, ok := store.parks[parkID]
    if !ok {
        http.Error(w, "park not found", http.StatusNotFound)
        return
    }

    for i := range p.Spots {
        if p.Spots[i].ID == spotID {
            spot := &p.Spots[i]
            
            // Check if OTP is empty (no reservation)
            if spot.OTP == "" {
                http.Error(w, "no active reservation", http.StatusBadRequest)
                return
            }
            
            // Check if OTP expired
            if time.Now().After(spot.OTPExpiry) {
                spot.OTP = ""
                spot.OTPExpiry = time.Time{}
                http.Error(w, "OTP expired", http.StatusUnauthorized)
                return
            }
            
            // Validate OTP
            if spot.OTP != pld.OTP {
                http.Error(w, "invalid OTP", http.StatusUnauthorized)
                return
            }
            
            // OTP is valid - clear it (single use) and trigger gate
            spot.OTP = ""
            spot.OTPExpiry = time.Time{}
            spot.LastOpen = time.Now().UTC().Format(time.RFC3339)
            
            // Send command to ESP32 to open gate
            if store.btBridge != nil {
                store.btBridge.DisplayOTP(fmt.Sprintf("OPEN:%d", spot.Number))
            }
            
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]any{
                "ok": true,
                "message": "Gate opened successfully",
            })
            return
        }
    }

    http.Error(w, "spot not found", http.StatusNotFound)
}

// clearOTPDisplayHandler clears the OTP from the ESP32 LCD screen
func clearOTPDisplayHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    parkID := vars["parkID"]
    spotID := vars["spotID"]

    store.mu.Lock()
    defer store.mu.Unlock()
    p, ok := store.parks[parkID]
    if !ok {
        http.Error(w, "park not found", http.StatusNotFound)
        return
    }

    // Verify spot exists
    found := false
    for i := range p.Spots {
        if p.Spots[i].ID == spotID {
            found = true
            break
        }
    }

    if !found {
        http.Error(w, "spot not found", http.StatusNotFound)
        return
    }

    // Send command to ESP32 to clear OTP display
    if store.btBridge != nil {
        err := store.btBridge.DisplayOTP("CLEAR")
        if err != nil {
            http.Error(w, "failed to send command to ESP32", http.StatusInternalServerError)
            return
        }
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "ok": true,
        "message": "OTP display cleared",
    })
}

// validateOTPViaESP32Handler sends OTP to ESP32 for local validation
func validateOTPViaESP32Handler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    parkID := vars["parkID"]
    spotID := vars["spotID"]

    type payload struct {
        OTP string `json:"otp"`
    }

    var pld payload
    if err := json.NewDecoder(r.Body).Decode(&pld); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }

    store.mu.Lock()
    defer store.mu.Unlock()
    p, ok := store.parks[parkID]
    if !ok {
        http.Error(w, "park not found", http.StatusNotFound)
        return
    }

    var spotNumber int
    found := false
    for i := range p.Spots {
        if p.Spots[i].ID == spotID {
            spotNumber = p.Spots[i].Number
            found = true
            break
        }
    }

    if !found {
        http.Error(w, "spot not found", http.StatusNotFound)
        return
    }

    // Send VALIDATE command to ESP32
    // Format: VALIDATE:spot:otp
    validateCmd := fmt.Sprintf("VALIDATE:%d:%s", spotNumber, pld.OTP)
    
    if store.btBridge != nil {
        err := store.btBridge.DisplayOTP(validateCmd)
        if err != nil {
            http.Error(w, "failed to send command to ESP32", http.StatusInternalServerError)
            return
        }
    } else {
        http.Error(w, "Bluetooth bridge not available", http.StatusServiceUnavailable)
        return
    }

    // Note: ESP32 will validate and respond with OTP_VALID or OTP_INVALID
    // The response will be handled by the message handler
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "ok": true,
        "message": "OTP validation sent to ESP32",
    })
}
