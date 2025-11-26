package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// Message types
const (
	TypeConnect        = "connect"
	TypeMatched        = "matched"
	TypeMessage        = "message"
	TypeDisconnect     = "disconnect"
	TypePartnerLeft    = "partner_left"
	TypeSearching      = "searching"
	TypeError          = "error"
	TypeTyping         = "typing"
	TypeStopTyping     = "stop_typing"
	TypeRestoreSession = "restore_session"
	TypeSessionExpired = "session_expired"
)

// Chat session status
const (
	SessionActive = "active"
	SessionEnded  = "ended"
)

// Message represents a WebSocket message
type Message struct {
	Type      string          `json:"type"`
	Content   string          `json:"content,omitempty"`
	Latitude  float64         `json:"latitude,omitempty"`
	Longitude float64         `json:"longitude,omitempty"`
	Timestamp int64           `json:"timestamp,omitempty"`
	DeviceID  string          `json:"device_id,omitempty"`
	Messages  []StoredMessage `json:"messages,omitempty"`
}

// StoredMessage represents a message stored in database
type StoredMessage struct {
	Content   string `json:"content"`
	IsSender  bool   `json:"is_sender"`
	Timestamp int64  `json:"timestamp"`
}

// User represents a connected user
type User struct {
	ID          string
	DeviceID    string
	Conn        *websocket.Conn
	Latitude    float64
	Longitude   float64
	PartnerID   string
	ChatSession string
	mu          sync.Mutex
}

// Hub manages all users and matching
type Hub struct {
	users       map[string]*User
	deviceToUser map[string]string // deviceID -> userID mapping
	waiting     []*User
	mu          sync.RWMutex
	maxDistance float64
	db          *pgxpool.Pool
}

func NewHub(db *pgxpool.Pool) *Hub {
	return &Hub{
		users:        make(map[string]*User),
		deviceToUser: make(map[string]string),
		waiting:      make([]*User, 0),
		maxDistance:  1000,
		db:           db,
	}
}

// Initialize database schema
func initDB(db *pgxpool.Pool) error {
	ctx := context.Background()

	schema := `
	CREATE TABLE IF NOT EXISTS devices (
		device_id VARCHAR(36) PRIMARY KEY,
		latitude DOUBLE PRECISION DEFAULT 0,
		longitude DOUBLE PRECISION DEFAULT 0,
		created_at TIMESTAMP DEFAULT NOW(),
		last_seen TIMESTAMP DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS chat_sessions (
		id VARCHAR(36) PRIMARY KEY,
		device_a VARCHAR(36) REFERENCES devices(device_id),
		device_b VARCHAR(36) REFERENCES devices(device_id),
		status VARCHAR(20) DEFAULT 'active',
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS messages (
		id SERIAL PRIMARY KEY,
		chat_session_id VARCHAR(36) REFERENCES chat_sessions(id),
		sender_device_id VARCHAR(36) REFERENCES devices(device_id),
		content TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_chat_sessions_device_a ON chat_sessions(device_a);
	CREATE INDEX IF NOT EXISTS idx_chat_sessions_device_b ON chat_sessions(device_b);
	CREATE INDEX IF NOT EXISTS idx_chat_sessions_status ON chat_sessions(status);
	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(chat_session_id);
	`

	_, err := db.Exec(ctx, schema)
	return err
}

// Register or update device in database
func (h *Hub) registerDevice(deviceID string, lat, lon float64) error {
	ctx := context.Background()
	_, err := h.db.Exec(ctx, `
		INSERT INTO devices (device_id, latitude, longitude, last_seen)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (device_id) DO UPDATE SET
			latitude = $2,
			longitude = $3,
			last_seen = NOW()
	`, deviceID, lat, lon)
	return err
}

// Find active chat session for device
func (h *Hub) findActiveSession(deviceID string) (string, string, error) {
	ctx := context.Background()
	var sessionID, partnerDeviceID string

	err := h.db.QueryRow(ctx, `
		SELECT id,
			CASE WHEN device_a = $1 THEN device_b ELSE device_a END as partner
		FROM chat_sessions
		WHERE (device_a = $1 OR device_b = $1)
			AND status = 'active'
			AND updated_at > NOW() - INTERVAL '30 minutes'
		ORDER BY updated_at DESC
		LIMIT 1
	`, deviceID).Scan(&sessionID, &partnerDeviceID)

	if err != nil {
		return "", "", err
	}
	return sessionID, partnerDeviceID, nil
}

// Create new chat session
func (h *Hub) createChatSession(deviceA, deviceB string) (string, error) {
	ctx := context.Background()
	sessionID := uuid.New().String()

	_, err := h.db.Exec(ctx, `
		INSERT INTO chat_sessions (id, device_a, device_b, status)
		VALUES ($1, $2, $3, 'active')
	`, sessionID, deviceA, deviceB)

	return sessionID, err
}

// End chat session
func (h *Hub) endChatSession(sessionID string) error {
	ctx := context.Background()
	_, err := h.db.Exec(ctx, `
		UPDATE chat_sessions SET status = 'ended', updated_at = NOW()
		WHERE id = $1
	`, sessionID)
	return err
}

// Update session timestamp (keep alive)
func (h *Hub) updateSessionTimestamp(sessionID string) error {
	ctx := context.Background()
	_, err := h.db.Exec(ctx, `
		UPDATE chat_sessions SET updated_at = NOW()
		WHERE id = $1
	`, sessionID)
	return err
}

// Store message in database
func (h *Hub) storeMessage(sessionID, senderDeviceID, content string) error {
	ctx := context.Background()
	_, err := h.db.Exec(ctx, `
		INSERT INTO messages (chat_session_id, sender_device_id, content)
		VALUES ($1, $2, $3)
	`, sessionID, senderDeviceID, content)

	// Also update session timestamp
	if err == nil {
		h.updateSessionTimestamp(sessionID)
	}
	return err
}

// Get messages for a chat session
func (h *Hub) getSessionMessages(sessionID, deviceID string) ([]StoredMessage, error) {
	ctx := context.Background()
	rows, err := h.db.Query(ctx, `
		SELECT content, sender_device_id, EXTRACT(EPOCH FROM created_at) * 1000
		FROM messages
		WHERE chat_session_id = $1
		ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []StoredMessage
	for rows.Next() {
		var content, senderID string
		var timestamp float64
		if err := rows.Scan(&content, &senderID, &timestamp); err != nil {
			continue
		}
		messages = append(messages, StoredMessage{
			Content:   content,
			IsSender:  senderID == deviceID,
			Timestamp: int64(timestamp),
		})
	}
	return messages, nil
}

// Calculate distance between two coordinates using Haversine formula
func (h *Hub) calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371

	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// Format distance for display (like Tinder)
func formatDistance(distance float64) string {
	if distance < 1 {
		return "< 1 km away"
	}
	return fmt.Sprintf("%.0f km away", distance)
}

// Find a match for the user
func (h *Hub) findMatch(user *User) *User {
	h.mu.Lock()
	defer h.mu.Unlock()

	var bestMatch *User
	bestDistance := math.MaxFloat64

	for i, waitingUser := range h.waiting {
		if waitingUser.DeviceID == user.DeviceID {
			continue
		}

		distance := h.calculateDistance(
			user.Latitude, user.Longitude,
			waitingUser.Latitude, waitingUser.Longitude,
		)

		if distance < bestDistance && distance <= h.maxDistance {
			bestMatch = waitingUser
			bestDistance = distance
			h.waiting = append(h.waiting[:i], h.waiting[i+1:]...)
			break
		}
	}

	if bestMatch == nil && len(h.waiting) > 0 {
		for i, waitingUser := range h.waiting {
			if waitingUser.DeviceID != user.DeviceID {
				bestMatch = waitingUser
				h.waiting = append(h.waiting[:i], h.waiting[i+1:]...)
				break
			}
		}
	}

	return bestMatch
}

// Add user to waiting list
func (h *Hub) addToWaiting(user *User) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, u := range h.waiting {
		if u.DeviceID == user.DeviceID {
			return
		}
	}

	h.waiting = append(h.waiting, user)
}

// Remove user from waiting list
func (h *Hub) removeFromWaiting(deviceID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, u := range h.waiting {
		if u.DeviceID == deviceID {
			h.waiting = append(h.waiting[:i], h.waiting[i+1:]...)
			break
		}
	}
}

// Register a new user
func (h *Hub) register(user *User) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.users[user.ID] = user
	if user.DeviceID != "" {
		h.deviceToUser[user.DeviceID] = user.ID
	}
}

// Unregister a user
func (h *Hub) unregister(userID string) {
	h.mu.Lock()
	user, exists := h.users[userID]
	if exists {
		delete(h.users, userID)
		if user.DeviceID != "" {
			delete(h.deviceToUser, user.DeviceID)
		}
	}
	h.mu.Unlock()

	if exists && user.PartnerID != "" {
		h.notifyPartnerLeft(user.PartnerID, user.ChatSession)
	}

	if user != nil {
		h.removeFromWaiting(user.DeviceID)
	}
}

// Get user by ID
func (h *Hub) getUser(userID string) *User {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.users[userID]
}

// Get user by device ID
func (h *Hub) getUserByDevice(deviceID string) *User {
	h.mu.RLock()
	userID, exists := h.deviceToUser[deviceID]
	h.mu.RUnlock()
	if !exists {
		return nil
	}
	return h.getUser(userID)
}

// Notify partner that user left
func (h *Hub) notifyPartnerLeft(partnerID string, sessionID string) {
	partner := h.getUser(partnerID)
	if partner != nil {
		partner.mu.Lock()
		partner.PartnerID = ""
		partner.ChatSession = ""
		partner.mu.Unlock()

		msg := Message{
			Type:      TypePartnerLeft,
			Timestamp: time.Now().UnixMilli(),
		}
		partner.sendMessage(msg)
	}

	// End the session in database
	if sessionID != "" {
		h.endChatSession(sessionID)
	}
}

// Match two users together
func (h *Hub) matchUsers(user1, user2 *User) {
	// Create chat session in database
	sessionID, err := h.createChatSession(user1.DeviceID, user2.DeviceID)
	if err != nil {
		log.Printf("Error creating chat session: %v", err)
		return
	}

	user1.mu.Lock()
	user1.PartnerID = user2.ID
	user1.ChatSession = sessionID
	user1.mu.Unlock()

	user2.mu.Lock()
	user2.PartnerID = user1.ID
	user2.ChatSession = sessionID
	user2.mu.Unlock()

	distance := h.calculateDistance(
		user1.Latitude, user1.Longitude,
		user2.Latitude, user2.Longitude,
	)

	distanceStr := formatDistance(distance)

	msg1 := Message{
		Type:      TypeMatched,
		Content:   distanceStr,
		Timestamp: time.Now().UnixMilli(),
	}
	msg2 := Message{
		Type:      TypeMatched,
		Content:   distanceStr,
		Timestamp: time.Now().UnixMilli(),
	}

	user1.sendMessage(msg1)
	user2.sendMessage(msg2)
}

// Send message to user
func (u *User) sendMessage(msg Message) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.Conn.WriteJSON(msg)
}

// Handle WebSocket connection
func (h *Hub) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	user := &User{
		ID:   uuid.New().String(),
		Conn: conn,
	}

	h.register(user)
	log.Printf("User connected: %s", user.ID)

	defer func() {
		h.unregister(user.ID)
		conn.Close()
		log.Printf("User disconnected: %s (device: %s)", user.ID, user.DeviceID)
	}()

	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		switch msg.Type {
		case TypeConnect:
			// Set device ID and location
			if msg.DeviceID == "" {
				msg.DeviceID = uuid.New().String()
			}

			user.DeviceID = msg.DeviceID
			user.Latitude = msg.Latitude
			user.Longitude = msg.Longitude

			// Update device to user mapping
			h.mu.Lock()
			h.deviceToUser[user.DeviceID] = user.ID
			h.mu.Unlock()

			// Register device in database
			h.registerDevice(user.DeviceID, user.Latitude, user.Longitude)

			// Check for existing active session
			sessionID, partnerDeviceID, err := h.findActiveSession(user.DeviceID)
			if err == nil && sessionID != "" {
				// Found active session, try to restore
				partner := h.getUserByDevice(partnerDeviceID)
				if partner != nil {
					// Partner is online, restore session
					user.mu.Lock()
					user.PartnerID = partner.ID
					user.ChatSession = sessionID
					user.mu.Unlock()

					partner.mu.Lock()
					partner.PartnerID = user.ID
					partner.ChatSession = sessionID
					partner.mu.Unlock()

					// Get previous messages
					messages, _ := h.getSessionMessages(sessionID, user.DeviceID)

					// Calculate distance for restored session
					distance := h.calculateDistance(
						user.Latitude, user.Longitude,
						partner.Latitude, partner.Longitude,
					)

					distanceStr := formatDistance(distance)

					// Send restore session message with distance
					user.sendMessage(Message{
						Type:      TypeRestoreSession,
						Content:   distanceStr,
						Timestamp: time.Now().UnixMilli(),
						Messages:  messages,
					})

					log.Printf("Restored session %s for device %s with distance: %s", sessionID, user.DeviceID, distanceStr)
					continue
				} else {
					// Partner is offline, session expired
					h.endChatSession(sessionID)
					user.sendMessage(Message{
						Type:      TypeSessionExpired,
						Content:   "Previous chat session expired",
						Timestamp: time.Now().UnixMilli(),
					})
				}
			}

			// If user has a current partner, disconnect first
			if user.PartnerID != "" {
				h.notifyPartnerLeft(user.PartnerID, user.ChatSession)
				user.mu.Lock()
				user.PartnerID = ""
				user.ChatSession = ""
				user.mu.Unlock()
			}

			// Try to find a match
			match := h.findMatch(user)
			if match != nil {
				h.matchUsers(user, match)
			} else {
				h.addToWaiting(user)
				user.sendMessage(Message{
					Type:      TypeSearching,
					Timestamp: time.Now().UnixMilli(),
				})
			}

		case TypeMessage:
			if user.PartnerID == "" {
				user.sendMessage(Message{
					Type:    TypeError,
					Content: "You are not connected to anyone",
				})
				continue
			}

			// Store message in database
			if user.ChatSession != "" {
				h.storeMessage(user.ChatSession, user.DeviceID, msg.Content)
			}

			partner := h.getUser(user.PartnerID)
			if partner != nil {
				partner.sendMessage(Message{
					Type:      TypeMessage,
					Content:   msg.Content,
					Timestamp: time.Now().UnixMilli(),
				})
			}

		case TypeTyping:
			if user.PartnerID != "" {
				partner := h.getUser(user.PartnerID)
				if partner != nil {
					partner.sendMessage(Message{
						Type:      TypeTyping,
						Timestamp: time.Now().UnixMilli(),
					})
				}
			}

		case TypeStopTyping:
			if user.PartnerID != "" {
				partner := h.getUser(user.PartnerID)
				if partner != nil {
					partner.sendMessage(Message{
						Type:      TypeStopTyping,
						Timestamp: time.Now().UnixMilli(),
					})
				}
			}

		case TypeDisconnect:
			if user.PartnerID != "" {
				h.notifyPartnerLeft(user.PartnerID, user.ChatSession)
				user.mu.Lock()
				user.PartnerID = ""
				user.ChatSession = ""
				user.mu.Unlock()
			}
			h.removeFromWaiting(user.DeviceID)
		}
	}
}

func main() {
	// Load .env file
	godotenv.Load()

	// Connect to database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(ctx); err != nil {
		log.Fatalf("Unable to ping database: %v", err)
	}
	log.Println("Connected to database")

	// Initialize schema
	if err := initDB(db); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}
	log.Println("Database schema initialized")

	hub := NewHub(db)

	http.HandleFunc("/ws", hub.handleWebSocket)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	log.Println("WebSocket server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
