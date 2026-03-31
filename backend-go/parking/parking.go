package parking

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type Spot struct {
	ID                 string    `json:"id"`
	Number             int       `json:"number"`
	Occupied           bool      `json:"occupied"`
	LastOpen           string    `json:"lastOpen,omitempty"`
	OTP                string    `json:"otp,omitempty"`
	OTPExpiry          time.Time `json:"otpExpiry,omitempty"`
	OwnerUserID        string    `json:"ownerUserId,omitempty"`
	OwnedByCurrentUser bool      `json:"ownedByCurrentUser"`
}

type Park struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Spots []Spot `json:"spots"`
}

type parkingEvent struct {
	Type        string `json:"type"`
	ParkID      string `json:"parkId"`
	Spot        Spot   `json:"spot"`
	ActorUserID string `json:"actorUserId,omitempty"`
	Timestamp   string `json:"timestamp"`
}

type parkingUserEvent struct {
	Type               string `json:"type"`
	ParkID             string `json:"parkId"`
	Spot               Spot   `json:"spot"`
	ActorIsCurrentUser bool   `json:"actorIsCurrentUser"`
	Timestamp          string `json:"timestamp"`
}

type parkingWebSocketClient struct {
	conn    *websocket.Conn
	userID  string
	writeMu sync.Mutex
}

type parkingWebSocketHub struct {
	mu      sync.RWMutex
	clients map[*parkingWebSocketClient]struct{}
	events  chan parkingEvent
}

func newParkingWebSocketHub() *parkingWebSocketHub {
	hub := &parkingWebSocketHub{
		clients: make(map[*parkingWebSocketClient]struct{}),
		events:  make(chan parkingEvent, 256),
	}
	go hub.run()
	return hub
}

func (h *parkingWebSocketHub) run() {
	for event := range h.events {
		h.broadcast(event)
	}
}

func (h *parkingWebSocketHub) addClient(conn *websocket.Conn, userID string) *parkingWebSocketClient {
	client := &parkingWebSocketClient{conn: conn, userID: userID}
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
	return client
}

func (h *parkingWebSocketHub) removeClient(client *parkingWebSocketClient) {
	h.mu.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		client.writeMu.Lock()
		_ = client.conn.Close()
		client.writeMu.Unlock()
	}
	h.mu.Unlock()
}

func (h *parkingWebSocketHub) readLoop(client *parkingWebSocketClient) {
	defer h.removeClient(client)
	for {
		if _, _, err := client.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *parkingWebSocketHub) publish(event parkingEvent) {
	select {
	case h.events <- event:
	default:
		log.Println("parking websocket queue full, dropping event")
	}
}

func (h *parkingWebSocketHub) broadcast(event parkingEvent) {
	h.mu.RLock()
	clients := make([]*parkingWebSocketClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		payload := parkingUserEvent{
			Type:               event.Type,
			ParkID:             event.ParkID,
			Spot:               spotForUser(event.Spot, client.userID),
			ActorIsCurrentUser: event.ActorUserID != "" && event.ActorUserID == client.userID,
			Timestamp:          event.Timestamp,
		}

		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			continue
		}

		client.writeMu.Lock()
		_ = client.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		err = client.conn.WriteMessage(websocket.TextMessage, jsonPayload)
		client.writeMu.Unlock()
		if err != nil {
			h.removeClient(client)
		}
	}
}

type ParkingStore struct {
	mu             sync.RWMutex
	reservationMu  sync.Mutex
	parks          map[string]*Park
	btBridge       BluetoothBridgeInterface
	db             *sql.DB
	userIDProvider UserIDProvider
	wsHub          *parkingWebSocketHub
	consoleOnce    sync.Once
}

type BluetoothBridgeInterface interface {
	DisplayOTP(otp string) error
}

type UserIDProvider interface {
	GetUserIDFromRequest(r *http.Request) (string, error)
}

var store *ParkingStore

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func SetBluetoothBridge(bridge BluetoothBridgeInterface) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.btBridge = bridge
}

func SetDB(db *sql.DB) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.db = db
	if err := store.ensureSchemaLocked(); err != nil {
		return err
	}
	if err := store.seedDefaultsLocked(); err != nil {
		return err
	}
	return store.loadFromDBLocked()
}

func SetUserIDProvider(provider UserIDProvider) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.userIDProvider = provider
}

func StartLiveConsole() {
	store.consoleOnce.Do(func() {
		go store.runConsoleLoop()
	})
}

func generateOTP() string {
	return fmt.Sprintf("%04d", rand.Intn(10000))
}

func init() {
	rand.Seed(time.Now().UnixNano())

	store = &ParkingStore{
		parks: make(map[string]*Park),
		wsHub: newParkingWebSocketHub(),
	}

	pid := "park-1"
	p := &Park{
		ID:    pid,
		Name:  "Parking Site 1",
		Spots: make([]Spot, 3),
	}
	for s := 0; s < 3; s++ {
		p.Spots[s] = Spot{
			ID:       fmt.Sprintf("%s-spot-%d", pid, s+1),
			Number:   s + 1,
			Occupied: false,
		}
	}
	store.parks[pid] = p
}

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
	r.HandleFunc("/ws/parking", parkingEventsWSHandler).Methods("GET")
}

func parkingEventsWSHandler(w http.ResponseWriter, r *http.Request) {
	if queryToken := strings.TrimSpace(r.URL.Query().Get("token")); queryToken != "" && strings.TrimSpace(r.Header.Get("Authorization")) == "" {
		r.Header.Set("Authorization", "Bearer "+queryToken)
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := store.wsHub.addClient(conn, userID)
	go store.wsHub.readLoop(client)
}

func listParksHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	store.mu.Lock()
	store.clearExpiredOTPsLocked()

	parks := make([]Park, 0, len(store.parks))
	for _, p := range store.parks {
		spots := make([]Spot, len(p.Spots))
		for i := range p.Spots {
			spots[i] = spotForUser(p.Spots[i], userID)
		}
		parks = append(parks, Park{
			ID:    p.ID,
			Name:  p.Name,
			Spots: spots,
		})
	}
	store.mu.Unlock()

	sort.Slice(parks, func(i, j int) bool {
		return parks[i].ID < parks[j].ID
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(parks)
}

type reserveResp struct {
	ParkID     string `json:"parkId"`
	SpotID     string `json:"spotId"`
	SpotNumber int    `json:"spotNumber"`
	OTP        string `json:"otp"`
	OTPExpiry  string `json:"otpExpiry"`
}

func reserveSpotHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	parkID := vars["parkID"]

	store.reservationMu.Lock()
	store.mu.Lock()

	p, ok := store.parks[parkID]
	if !ok {
		store.mu.Unlock()
		store.reservationMu.Unlock()
		http.Error(w, "park not found", http.StatusNotFound)
		return
	}

	store.clearExpiredOTPsLocked()

	var chosen *Spot
	for i := range p.Spots {
		if p.Spots[i].OwnerUserID == userID {
			chosen = &p.Spots[i]
			break
		}
	}

	if chosen == nil {
		for i := range p.Spots {
			if !p.Spots[i].Occupied {
				chosen = &p.Spots[i]
				chosen.Occupied = true
				chosen.OwnerUserID = userID
				break
			}
		}
	}

	if chosen == nil {
		store.mu.Unlock()
		store.reservationMu.Unlock()
		http.Error(w, "no free spots", http.StatusConflict)
		return
	}

	store.ensureActiveOTPLocked(p, chosen)
	store.displaySpotOTPLocked(*chosen)
	response := toReserveResp(parkID, *chosen)
	store.publishSpotEventLocked("spot_reserved", p.ID, *chosen, userID)

	store.mu.Unlock()
	store.reservationMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func reserveSpecificSpotHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	parkID := vars["parkID"]
	spotID := vars["spotID"]

	store.reservationMu.Lock()
	store.mu.Lock()

	p, ok := store.parks[parkID]
	if !ok {
		store.mu.Unlock()
		store.reservationMu.Unlock()
		http.Error(w, "park not found", http.StatusNotFound)
		return
	}

	store.clearExpiredOTPsLocked()

	spot := findSpotByID(p, spotID)
	if spot == nil {
		store.mu.Unlock()
		store.reservationMu.Unlock()
		http.Error(w, "spot not found", http.StatusNotFound)
		return
	}

	if spot.Occupied && spot.OwnerUserID != userID {
		store.mu.Unlock()
		store.reservationMu.Unlock()
		http.Error(w, "spot already occupied by another user", http.StatusConflict)
		return
	}

	spot.Occupied = true
	spot.OwnerUserID = userID
	store.ensureActiveOTPLocked(p, spot)
	store.displaySpotOTPLocked(*spot)
	response := toReserveResp(parkID, *spot)
	store.publishSpotEventLocked("spot_reserved", p.ID, *spot, userID)

	store.mu.Unlock()
	store.reservationMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func releaseSpotHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

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

	spot := findSpotByID(p, spotID)
	if spot == nil {
		http.Error(w, "spot not found", http.StatusNotFound)
		return
	}

	if !spot.Occupied || spot.OwnerUserID != userID {
		http.Error(w, "you do not own this spot", http.StatusForbidden)
		return
	}

	spot.Occupied = false
	spot.OwnerUserID = ""
	spot.OTP = ""
	spot.OTPExpiry = time.Time{}
	spot.LastOpen = ""
	store.saveSpotLocked(p, *spot)
	store.publishSpotEventLocked("spot_released", p.ID, *spot, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

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

	spot := findSpotByID(p, spotID)
	if spot == nil {
		http.Error(w, "spot not found", http.StatusNotFound)
		return
	}

	spot.Occupied = pld.Occupied
	if !pld.Occupied {
		spot.OwnerUserID = ""
		spot.OTP = ""
		spot.OTPExpiry = time.Time{}
	}
	store.saveSpotLocked(p, *spot)
	store.publishSpotEventLocked("spot_updated", p.ID, *spot, "")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func openGateHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

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

	spot := findSpotByID(p, spotID)
	if spot == nil {
		http.Error(w, "spot not found", http.StatusNotFound)
		return
	}

	if spot.OwnerUserID != userID {
		http.Error(w, "you do not own this spot", http.StatusForbidden)
		return
	}

	spot.LastOpen = time.Now().UTC().Format(time.RFC3339)
	store.saveSpotLocked(p, *spot)

	if store.btBridge != nil {
		cmd := fmt.Sprintf("toggle %d", spot.Number)
		if err := store.btBridge.DisplayOTP(cmd); err != nil {
			http.Error(w, fmt.Sprintf("failed to toggle gate: %v", err), http.StatusInternalServerError)
			return
		}
	}

	store.publishSpotEventLocked("spot_gate_toggled", p.ID, *spot, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "message": "Gate toggled"})
}

func validateOTPHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

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
	pld.OTP = strings.TrimSpace(pld.OTP)
	if !isFourDigitOTP(pld.OTP) {
		http.Error(w, "invalid OTP format", http.StatusBadRequest)
		return
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	p, ok := store.parks[parkID]
	if !ok {
		http.Error(w, "park not found", http.StatusNotFound)
		return
	}

	spot := findSpotByID(p, spotID)
	if spot == nil {
		http.Error(w, "spot not found", http.StatusNotFound)
		return
	}

	if spot.OwnerUserID != userID {
		http.Error(w, "you do not own this spot", http.StatusForbidden)
		return
	}

	if spot.OTP == "" {
		http.Error(w, "no active OTP", http.StatusBadRequest)
		return
	}

	if time.Now().After(spot.OTPExpiry) {
		spot.OTP = ""
		spot.OTPExpiry = time.Time{}
		store.saveSpotLocked(p, *spot)
		http.Error(w, "OTP expired", http.StatusUnauthorized)
		return
	}

	if spot.OTP != pld.OTP {
		http.Error(w, "invalid OTP", http.StatusUnauthorized)
		return
	}

	if store.btBridge != nil {
		if err := store.btBridge.DisplayOTP(fmt.Sprintf("OPEN:%d", spot.Number)); err != nil {
			http.Error(w, fmt.Sprintf("failed to open gate on ESP32: %v", err), http.StatusServiceUnavailable)
			return
		}
	}

	spot.OTP = ""
	spot.OTPExpiry = time.Time{}
	spot.LastOpen = time.Now().UTC().Format(time.RFC3339)
	store.saveSpotLocked(p, *spot)

	store.publishSpotEventLocked("spot_otp_validated", p.ID, *spot, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "Gate opened successfully",
	})
}

func clearOTPDisplayHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

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

	spot := findSpotByID(p, spotID)
	if spot == nil {
		http.Error(w, "spot not found", http.StatusNotFound)
		return
	}

	if spot.OwnerUserID != userID {
		http.Error(w, "you do not own this spot", http.StatusForbidden)
		return
	}

	if store.btBridge != nil {
		if err := store.btBridge.DisplayOTP("CLEAR"); err != nil {
			http.Error(w, "failed to send command to ESP32", http.StatusInternalServerError)
			return
		}
	}

	store.publishSpotEventLocked("spot_lcd_cleared", p.ID, *spot, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "OTP display cleared",
	})
}

func validateOTPViaESP32Handler(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

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
	pld.OTP = strings.TrimSpace(pld.OTP)
	if !isFourDigitOTP(pld.OTP) {
		http.Error(w, "invalid OTP format", http.StatusBadRequest)
		return
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	p, ok := store.parks[parkID]
	if !ok {
		http.Error(w, "park not found", http.StatusNotFound)
		return
	}

	spot := findSpotByID(p, spotID)
	if spot == nil {
		http.Error(w, "spot not found", http.StatusNotFound)
		return
	}

	if spot.OwnerUserID != userID {
		http.Error(w, "you do not own this spot", http.StatusForbidden)
		return
	}

	if store.btBridge == nil {
		http.Error(w, "Bluetooth bridge not available", http.StatusServiceUnavailable)
		return
	}

	validateCmd := fmt.Sprintf("VALIDATE:%d:%s", spot.Number, pld.OTP)
	if err := store.btBridge.DisplayOTP(validateCmd); err != nil {
		http.Error(w, "failed to send command to ESP32", http.StatusInternalServerError)
		return
	}

	store.publishSpotEventLocked("spot_otp_validation_sent", p.ID, *spot, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "OTP validation sent to ESP32",
	})
}

func findSpotByID(p *Park, spotID string) *Spot {
	for i := range p.Spots {
		if p.Spots[i].ID == spotID {
			return &p.Spots[i]
		}
	}
	return nil
}

func toReserveResp(parkID string, spot Spot) reserveResp {
	return reserveResp{
		ParkID:     parkID,
		SpotID:     spot.ID,
		SpotNumber: spot.Number,
		OTP:        spot.OTP,
		OTPExpiry:  spot.OTPExpiry.Format(time.RFC3339),
	}
}

func spotForUser(spot Spot, userID string) Spot {
	view := spot
	view.OwnedByCurrentUser = view.OwnerUserID != "" && view.OwnerUserID == userID
	if !view.OwnedByCurrentUser {
		view.OTP = ""
		view.OTPExpiry = time.Time{}
	}
	return view
}

func isFourDigitOTP(value string) bool {
	if len(value) != 4 {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func requireUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	store.mu.RLock()
	provider := store.userIDProvider
	store.mu.RUnlock()

	if provider == nil {
		http.Error(w, "auth unavailable", http.StatusInternalServerError)
		return "", false
	}

	userID, err := provider.GetUserIDFromRequest(r)
	if err != nil || strings.TrimSpace(userID) == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", false
	}

	return userID, true
}

func (ps *ParkingStore) ensureActiveOTPLocked(p *Park, spot *Spot) {
	now := time.Now()
	if spot.OTP != "" && now.Before(spot.OTPExpiry) {
		ps.saveSpotLocked(p, *spot)
		return
	}

	spot.OTP = generateOTP()
	spot.OTPExpiry = now.Add(15 * time.Minute)
	ps.saveSpotLocked(p, *spot)
}

func (ps *ParkingStore) displaySpotOTPLocked(spot Spot) {
	if ps.btBridge == nil {
		return
	}
	if spot.OTP == "" {
		return
	}
	ps.btBridge.DisplayOTP(fmt.Sprintf("%d:%s", spot.Number, spot.OTP))
}

func (ps *ParkingStore) clearExpiredOTPsLocked() {
	now := time.Now()
	for _, p := range ps.parks {
		for i := range p.Spots {
			if p.Spots[i].OTP == "" {
				continue
			}
			if now.After(p.Spots[i].OTPExpiry) {
				p.Spots[i].OTP = ""
				p.Spots[i].OTPExpiry = time.Time{}
				ps.saveSpotLocked(p, p.Spots[i])
			}
		}
	}
}

func (ps *ParkingStore) publishSpotEventLocked(eventType, parkID string, spot Spot, actorUserID string) {
	if ps.wsHub == nil {
		return
	}
	ps.wsHub.publish(parkingEvent{
		Type:        eventType,
		ParkID:      parkID,
		Spot:        spot,
		ActorUserID: actorUserID,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	})
}

func (ps *ParkingStore) ensureSchemaLocked() error {
	if ps.db == nil {
		return nil
	}

	query := `
	CREATE TABLE IF NOT EXISTS parking_spots (
		spot_id TEXT PRIMARY KEY,
		park_id TEXT NOT NULL,
		park_name TEXT NOT NULL,
		spot_number INTEGER NOT NULL,
		occupied INTEGER NOT NULL DEFAULT 0,
		owner_user_id TEXT NOT NULL DEFAULT '',
		otp TEXT NOT NULL DEFAULT '',
		otp_expiry TEXT NOT NULL DEFAULT '',
		last_open TEXT NOT NULL DEFAULT ''
	);`
	_, err := ps.db.Exec(query)
	return err
}

func (ps *ParkingStore) seedDefaultsLocked() error {
	if ps.db == nil {
		return nil
	}

	for _, p := range ps.parks {
		for _, s := range p.Spots {
			if _, err := ps.db.Exec(
				`INSERT OR IGNORE INTO parking_spots
				(spot_id, park_id, park_name, spot_number, occupied, owner_user_id, otp, otp_expiry, last_open)
				VALUES (?, ?, ?, ?, 0, '', '', '', '')`,
				s.ID, p.ID, p.Name, s.Number,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func (ps *ParkingStore) loadFromDBLocked() error {
	if ps.db == nil {
		return nil
	}

	rows, err := ps.db.Query(`
		SELECT park_id, park_name, spot_id, spot_number, occupied, owner_user_id, otp, otp_expiry, last_open
		FROM parking_spots
		ORDER BY park_id, spot_number`)
	if err != nil {
		return err
	}
	defer rows.Close()

	loaded := make(map[string]*Park)
	for rows.Next() {
		var parkID, parkName, spotID, ownerUserID, otp, otpExpiryStr, lastOpen string
		var spotNumber, occupiedInt int
		if err := rows.Scan(&parkID, &parkName, &spotID, &spotNumber, &occupiedInt, &ownerUserID, &otp, &otpExpiryStr, &lastOpen); err != nil {
			return err
		}

		park := loaded[parkID]
		if park == nil {
			park = &Park{ID: parkID, Name: parkName, Spots: make([]Spot, 0, 4)}
			loaded[parkID] = park
		}

		otpExpiry := time.Time{}
		if otpExpiryStr != "" {
			if parsed, parseErr := time.Parse(time.RFC3339, otpExpiryStr); parseErr == nil {
				otpExpiry = parsed
			}
		}

		park.Spots = append(park.Spots, Spot{
			ID:          spotID,
			Number:      spotNumber,
			Occupied:    occupiedInt == 1,
			OwnerUserID: ownerUserID,
			OTP:         otp,
			OTPExpiry:   otpExpiry,
			LastOpen:    lastOpen,
		})
	}

	if err := rows.Err(); err != nil {
		return err
	}

	if len(loaded) > 0 {
		for _, p := range loaded {
			sort.Slice(p.Spots, func(i, j int) bool {
				return p.Spots[i].Number < p.Spots[j].Number
			})
		}
		ps.parks = loaded
	}

	ps.clearExpiredOTPsLocked()
	return nil
}

func (ps *ParkingStore) saveSpotLocked(p *Park, spot Spot) {
	if ps.db == nil {
		return
	}

	otpExpiry := ""
	if !spot.OTPExpiry.IsZero() {
		otpExpiry = spot.OTPExpiry.UTC().Format(time.RFC3339)
	}

	_, _ = ps.db.Exec(
		`INSERT INTO parking_spots
		(spot_id, park_id, park_name, spot_number, occupied, owner_user_id, otp, otp_expiry, last_open)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(spot_id) DO UPDATE SET
		park_id=excluded.park_id,
		park_name=excluded.park_name,
		spot_number=excluded.spot_number,
		occupied=excluded.occupied,
		owner_user_id=excluded.owner_user_id,
		otp=excluded.otp,
		otp_expiry=excluded.otp_expiry,
		last_open=excluded.last_open`,
		spot.ID,
		p.ID,
		p.Name,
		spot.Number,
		boolToInt(spot.Occupied),
		spot.OwnerUserID,
		spot.OTP,
		otpExpiry,
		spot.LastOpen,
	)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (ps *ParkingStore) runConsoleLoop() {
	log.Println("Parking live console enabled. Commands: list | free <spot-id|spot-number> | free <park-id> <spot-id|spot-number> | help")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		ps.executeConsoleCommand(line)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("parking live console stopped with error: %v", err)
		return
	}
	log.Println("parking live console stopped")
}

func (ps *ParkingStore) executeConsoleCommand(line string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])
	switch cmd {
	case "help":
		log.Println("Commands: list | free <spot-id|spot-number> | free <park-id> <spot-id|spot-number>")
	case "list":
		ps.printConsoleState()
	case "free":
		if len(parts) == 2 {
			if err := ps.forceReleaseFromConsole("", parts[1]); err != nil {
				log.Printf("console free failed: %v", err)
			}
			return
		}
		if len(parts) == 3 {
			if err := ps.forceReleaseFromConsole(parts[1], parts[2]); err != nil {
				log.Printf("console free failed: %v", err)
			}
			return
		}
		log.Println("usage: free <spot-id|spot-number> OR free <park-id> <spot-id|spot-number>")
	default:
		log.Printf("unknown console command: %s", cmd)
	}
}

func (ps *ParkingStore) printConsoleState() {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	for _, park := range ps.parks {
		log.Printf("%s (%s)", park.Name, park.ID)
		for _, spot := range park.Spots {
			status := "free"
			if spot.Occupied {
				status = "occupied"
			}
			log.Printf("  spot=%d id=%s status=%s owner=%s", spot.Number, spot.ID, status, spot.OwnerUserID)
		}
	}
}

func (ps *ParkingStore) forceReleaseFromConsole(parkRef, spotRef string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	park, spot := ps.findSpotByReferenceLocked(parkRef, spotRef)
	if park == nil || spot == nil {
		return fmt.Errorf("spot not found for reference %q", spotRef)
	}

	spot.Occupied = false
	spot.OwnerUserID = ""
	spot.OTP = ""
	spot.OTPExpiry = time.Time{}
	spot.LastOpen = ""
	ps.saveSpotLocked(park, *spot)
	ps.publishSpotEventLocked("spot_console_freed", park.ID, *spot, "console")

	log.Printf("console freed spot %s in park %s", spot.ID, park.ID)
	return nil
}

func (ps *ParkingStore) findSpotByReferenceLocked(parkRef, spotRef string) (*Park, *Spot) {
	findByNumber := false
	spotNumber := 0
	if parsed, err := strconv.Atoi(strings.TrimSpace(spotRef)); err == nil {
		findByNumber = true
		spotNumber = parsed
	}

	if parkRef != "" {
		park := ps.parks[parkRef]
		if park == nil {
			return nil, nil
		}
		for i := range park.Spots {
			if findByNumber {
				if park.Spots[i].Number == spotNumber {
					return park, &park.Spots[i]
				}
				continue
			}
			if park.Spots[i].ID == spotRef {
				return park, &park.Spots[i]
			}
		}
		return nil, nil
	}

	for _, park := range ps.parks {
		for i := range park.Spots {
			if findByNumber {
				if park.Spots[i].Number == spotNumber {
					return park, &park.Spots[i]
				}
				continue
			}
			if park.Spots[i].ID == spotRef {
				return park, &park.Spots[i]
			}
		}
	}

	return nil, nil
}
