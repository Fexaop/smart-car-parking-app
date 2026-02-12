package parking

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"sync"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/gorilla/mux"
)

type Spot struct {
    ID       string `json:"id"`
    Number   int    `json:"number"`
    Occupied bool   `json:"occupied"`
    LastOpen string `json:"lastOpen,omitempty"`
    QRCode   string `json:"qrCode,omitempty"`
}

type Park struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Spots []Spot `json:"spots"`
}

type ParkingStore struct {
    mu sync.Mutex
    parks map[string]*Park
}

var store *ParkingStore

func init() {
    store = &ParkingStore{
        parks: make(map[string]*Park),
    }

    // initialize 4 parking sites with 5 spots each
    for i := 1; i <= 4; i++ {
        pid := fmt.Sprintf("park-%d", i)
        p := &Park{
            ID: pid,
            Name: fmt.Sprintf("Parking Site %d", i),
            Spots: make([]Spot, 5),
        }
        for s := 0; s < 5; s++ {
            p.Spots[s] = Spot{
                ID: fmt.Sprintf("%s-spot-%d", pid, s+1),
                Number: s + 1,
                Occupied: false,
            }
        }
        store.parks[pid] = p
    }
}

// RegisterRoutes registers HTTP handlers under /api/parks
func RegisterRoutes(r *mux.Router) {
    r.HandleFunc("/api/parks", listParksHandler).Methods("GET")
    r.HandleFunc("/api/parks/{parkID}/reserve", reserveSpotHandler).Methods("POST")
    r.HandleFunc("/api/parks/{parkID}/spots/{spotID}/reserve", reserveSpecificSpotHandler).Methods("POST")
    r.HandleFunc("/api/parks/{parkID}/spots/{spotID}/release", releaseSpotHandler).Methods("POST")
    r.HandleFunc("/api/parks/{parkID}/spots/{spotID}/update", updateSpotHandler).Methods("POST")
    r.HandleFunc("/api/parks/{parkID}/spots/{spotID}/opengate", openGateHandler).Methods("POST")
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
    QRBase64 string `json:"qrBase64"`
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

    // generate QR payload
    payload := map[string]any{
        "parkId": parkID,
        "spotId": chosen.ID,
        "spotNumber": chosen.Number,
        "reservedAt": time.Now().UTC().Format(time.RFC3339),
    }
    payloadBytes, _ := json.Marshal(payload)

    // make QR PNG using boombuler/barcode
    code, err := qr.Encode(string(payloadBytes), qr.M, qr.Auto)
    if err != nil {
        http.Error(w, "failed to generate qr", http.StatusInternalServerError)
        return
    }
    code, err = barcode.Scale(code, 256, 256)
    if err != nil {
        http.Error(w, "failed to scale qr", http.StatusInternalServerError)
        return
    }
    var buf bytes.Buffer
    if err := png.Encode(&buf, code); err != nil {
        http.Error(w, "failed to encode qr", http.StatusInternalServerError)
        return
    }
    b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
    
    // Store QR code in the spot
    chosen.QRCode = b64

    resp := reserveResp{
        ParkID: parkID,
        SpotID: chosen.ID,
        SpotNumber: chosen.Number,
        QRBase64: b64,
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

    // generate QR payload
    payload := map[string]any{
        "parkId": parkID,
        "spotId": chosen.ID,
        "spotNumber": chosen.Number,
        "reservedAt": time.Now().UTC().Format(time.RFC3339),
    }
    payloadBytes, _ := json.Marshal(payload)

    // make QR PNG using boombuler/barcode
    code, err := qr.Encode(string(payloadBytes), qr.M, qr.Auto)
    if err != nil {
        http.Error(w, "failed to generate qr", http.StatusInternalServerError)
        return
    }
    code, err = barcode.Scale(code, 256, 256)
    if err != nil {
        http.Error(w, "failed to scale qr", http.StatusInternalServerError)
        return
    }
    var buf bytes.Buffer
    if err := png.Encode(&buf, code); err != nil {
        http.Error(w, "failed to encode qr", http.StatusInternalServerError)
        return
    }
    b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
    
    // Store QR code in the spot
    chosen.QRCode = b64

    resp := reserveResp{
        ParkID: parkID,
        SpotID: chosen.ID,
        SpotNumber: chosen.Number,
        QRBase64: b64,
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
            p.Spots[i].QRCode = ""
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
                p.Spots[i].QRCode = ""
            }
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]any{"ok": true})
            return
        }
    }

    http.Error(w, "spot not found", http.StatusNotFound)
}

// openGateHandler records a gate-open event for a spot (simple notification)
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
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]any{"ok": true})
            return
        }
    }

    http.Error(w, "spot not found", http.StatusNotFound)
}
