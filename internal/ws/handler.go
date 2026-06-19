package ws

import (
	"cinemabooking/internal/domain"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// tighten this in production to check allowed origins
		return true
	},
}

type Handler struct {
	hub *Hub
}

func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

// ServeWS handles WS /ws/showtimes/:id
func (h *Handler) ServeWS(c *gin.Context) {
	showtimeID := c.Param("id")
	if showtimeID == "" {
		c.JSON(http.StatusBadRequest, domain.Fail[any]("showtime_id required"))
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("ws upgrade failed", "error", err)
		return
	}

	client := &Client{
		ShowtimeID: showtimeID,
		Send:       make(chan []byte, 64),
	}

	h.hub.register <- client

	// run read and write pumps in separate goroutines
	go h.writePump(client, conn)
	go h.readPump(client, conn)
}

// readPump keeps the connection alive by reading pong frames.
// We don't expect clients to send data — just pings.
func (h *Handler) readPump(client *Client, conn *websocket.Conn) {
	defer func() {
		h.hub.unregister <- client
		conn.Close()
	}()

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				slog.Error("ws read error", "error", err)
			}
			break
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (h *Handler) writePump(client *Client, conn *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case msg, ok := <-client.Send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// hub closed the channel
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				slog.Error("ws write error", "error", err)
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}