package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
)

type Client struct {
	ShowtimeID string
	Send       chan []byte
}

type RoomMessage struct {
	ShowtimeID string
	Data       []byte
}

type Hub struct {
	rooms      map[string]map[*Client]bool
	broadcast  chan RoomMessage
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		broadcast:  make(chan RoomMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.rooms[client.ShowtimeID] == nil {
				h.rooms[client.ShowtimeID] = make(map[*Client]bool)
			}
			h.rooms[client.ShowtimeID][client] = true
			h.mu.Unlock()
			slog.Info("ws client connected", "showtime_id", client.ShowtimeID)

		case client := <-h.unregister:
			h.mu.Lock()
			if room, ok := h.rooms[client.ShowtimeID]; ok {
				if _, ok := room[client]; ok {
					delete(room, client)
					close(client.Send)
					if len(room) == 0 {
						delete(h.rooms, client.ShowtimeID)
					}
				}
			}
			h.mu.Unlock()
			slog.Info("ws client disconnected", "showtime_id", client.ShowtimeID)

		case msg := <-h.broadcast:
			h.mu.RLock()
			room := h.rooms[msg.ShowtimeID]
			h.mu.RUnlock()

			for client := range room {
				select {
				case client.Send <- msg.Data:
				default:
					// client send buffer full — drop and disconnect
					h.unregister <- client
				}
			}
		}
	}
}

// Broadcast sends a typed event to all clients watching a showtime.
func (h *Hub) Broadcast(showtimeID string, event any) {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("ws marshal error", "error", err)
		return
	}
	h.broadcast <- RoomMessage{ShowtimeID: showtimeID, Data: data}
}

// ClientCount returns how many clients are watching a showtime.
func (h *Hub) ClientCount(showtimeID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[showtimeID])
}
