package main

import (
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// relayRoom fans out JPEG frames published by the device that owns a camera
// (its browser has the physical webcam) to every other browser watching
// that same camera id, since getUserMedia can only ever see local hardware.
type relayRoom struct {
	mu        sync.Mutex
	watchers  map[*websocket.Conn]*sync.Mutex
	lastFrame []byte
}

type relayHub struct {
	mu    sync.Mutex
	rooms map[string]*relayRoom
}

func newRelayHub() *relayHub {
	return &relayHub{rooms: map[string]*relayRoom{}}
}

func (h *relayHub) room(id string) *relayRoom {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[id]
	if !ok {
		r = &relayRoom{watchers: map[*websocket.Conn]*sync.Mutex{}}
		h.rooms[id] = r
	}
	return r
}

func (r *relayRoom) broadcast(frame []byte) {
	r.mu.Lock()
	r.lastFrame = frame
	watchers := make(map[*websocket.Conn]*sync.Mutex, len(r.watchers))
	for c, m := range r.watchers {
		watchers[c] = m
	}
	r.mu.Unlock()
	for c, m := range watchers {
		m.Lock()
		_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		err := c.WriteMessage(websocket.BinaryMessage, frame)
		m.Unlock()
		if err != nil {
			_ = c.Close()
		}
	}
}

func (r *relayRoom) addWatcher(c *websocket.Conn) *sync.Mutex {
	m := &sync.Mutex{}
	r.mu.Lock()
	r.watchers[c] = m
	last := r.lastFrame
	r.mu.Unlock()
	if last != nil {
		m.Lock()
		_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_ = c.WriteMessage(websocket.BinaryMessage, last)
		m.Unlock()
	}
	return m
}

func (r *relayRoom) removeWatcher(c *websocket.Conn) {
	r.mu.Lock()
	delete(r.watchers, c)
	r.mu.Unlock()
}

const maxRelayFrame = 512 * 1024

var relayUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		return err == nil && u.Host == r.Host
	},
}

func (a *App) relayRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/cameras/{id}/publish", a.publishCamera)
	m.HandleFunc("GET /api/v1/cameras/{id}/watch", a.watchCamera)
}

// publishCamera receives JPEG frames from the device that physically owns
// this camera and rebroadcasts each one to every connected watcher.
func (a *App) publishCamera(w http.ResponseWriter, r *http.Request) {
	conn, err := relayUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	room := a.relay.room(r.PathValue("id"))
	conn.SetReadLimit(maxRelayFrame)
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		return nil
	})
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if mt == websocket.BinaryMessage && len(data) > 0 {
			room.broadcast(data)
		}
	}
}

// watchCamera streams whatever frames publishCamera receives for this
// camera id to a browser that cannot open the physical device itself
// (e.g. viewing a phone's camera from a laptop, or vice versa).
func (a *App) watchCamera(w http.ResponseWriter, r *http.Request) {
	conn, err := relayUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	room := a.relay.room(r.PathValue("id"))
	room.addWatcher(conn)
	defer room.removeWatcher(conn)
	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
