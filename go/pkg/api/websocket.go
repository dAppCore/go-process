// SPDX-Licence-Identifier: EUPL-1.2

package api

import (
	"context"
	"net/http"
	"reflect"
	// Note: AX-6 — internal concurrency primitive; structural per RFC §2
	"sync"
	"time"

	core "dappco.re/go"
	"github.com/gorilla/websocket"
)

// messageType identifies the type of WebSocket message.
type messageType string

const (
	// typeEvent indicates a generic event.
	typeEvent messageType = "event"
)

// hubMessage is the standard WebSocket event payload emitted by the provider.
type hubMessage struct {
	Type      messageType `json:"type"`
	Channel   string      `json:"channel,omitempty"`
	ProcessID string      `json:"processId,omitempty"`
	Data      any         `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// hub is a small local WebSocket hub for tests and standalone process users.
type hub struct {
	clients    map[*websocketClient]bool
	register   chan *websocketClient
	unregister chan *websocketClient
	broadcast  chan []byte
	done       chan struct{}
	doneOnce   sync.Once
	running    bool
	mu         sync.RWMutex
}

type websocketClient struct {
	hub       *hub
	conn      *websocket.Conn
	send      chan []byte
	closeOnce sync.Once
}

// newHub constructs a local WebSocket hub.
func newHub() *hub {
	return &hub{
		clients:    make(map[*websocketClient]bool),
		register:   make(chan *websocketClient),
		unregister: make(chan *websocketClient),
		broadcast:  make(chan []byte, 256),
		done:       make(chan struct{}),
	}
}

// run starts the hub loop and exits when ctx is cancelled.
func (h *hub) run(ctx context.Context) {
	if h == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	h.mu.Lock()
	h.running = true
	h.mu.Unlock()
	defer h.doneOnce.Do(func() { close(h.done) })
	defer func() {
		h.mu.Lock()
		h.running = false
		h.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			h.closeAll()
			return
		case client := <-h.register:
			if client == nil {
				continue
			}
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.removeClient(client)
		case payload := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- payload:
				default:
					go h.removeClient(client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Handler returns an HTTP handler for WebSocket upgrade requests.
func (h *hub) handler() http.HandlerFunc {
	if h == nil {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "hub is not configured", http.StatusServiceUnavailable)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if !h.isRunning() {
			http.Error(w, "hub is not running", http.StatusServiceUnavailable)
			return
		}

		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(*http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		client := &websocketClient{
			hub:  h,
			conn: conn,
			send: make(chan []byte, 256),
		}

		select {
		case h.register <- client:
		case <-h.done:
			closeWebSocket(conn)
			return
		}

		go client.writePump()
		go client.readPump()
	}
}

func (h *hub) broadcastMessage(msg hubMessage) core.Result {
	if h == nil {
		return core.Fail(core.E("hub.broadcastMessage", "hub is nil", nil))
	}
	msg.Timestamp = time.Now()
	encoded := core.JSONMarshal(msg)
	if !encoded.OK {
		err, _ := encoded.Value.(error)
		return core.Fail(core.E("hub.broadcastMessage", "failed to marshal message", err))
	}

	select {
	case h.broadcast <- encoded.Value.([]byte):
	default:
		return core.Fail(core.E("hub.broadcastMessage", "broadcast channel full", nil))
	}
	return core.Ok(nil)
}

func (h *hub) sendToChannelMessage(string, hubMessage) core.Result {
	return core.Ok(nil)
}

// ClientCount returns the number of connected WebSocket clients.
func (h *hub) clientCount() int {
	if h == nil {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *hub) isRunning() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.running
}

func (h *hub) closeAll() {
	h.mu.Lock()
	clients := make([]*websocketClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
		delete(h.clients, client)
	}
	h.mu.Unlock()
	for _, client := range clients {
		client.close()
	}
}

func (h *hub) removeClient(client *websocketClient) {
	if h == nil || client == nil {
		return
	}
	h.mu.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		h.mu.Unlock()
		client.close()
		return
	}
	h.mu.Unlock()
}

func (c *websocketClient) readPump() {
	if c == nil || c.conn == nil {
		return
	}
	defer func() {
		if c.hub != nil {
			select {
			case c.hub.unregister <- c:
			case <-c.hub.done:
			}
		}
		closeWebSocket(c.conn)
	}()

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *websocketClient) writePump() {
	if c == nil || c.conn == nil {
		return
	}
	defer closeWebSocket(c.conn)

	for payload := range c.send {
		if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return
		}
		if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			return
		}
	}
}

func (c *websocketClient) close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(func() {
		close(c.send)
		if c.conn != nil {
			closeWebSocket(c.conn)
		}
	})
}

func closeWebSocket(conn *websocket.Conn) {
	if conn == nil {
		return
	}
	if err := conn.Close(); err != nil {
		return
	}
}

func emitHubEvent(target any, channel string, data any) {
	msg := hubMessage{
		Type:    typeEvent,
		Channel: channel,
		Data:    data,
	}

	if local, ok := target.(*hub); ok {
		if result := local.broadcastMessage(msg); !result.OK {
			return
		}
		if result := local.sendToChannelMessage(channel, hubMessage{Type: typeEvent, Data: data}); !result.OK {
			return
		}
		return
	}

	callExternalHub(target, "Broadcast", channel, msg)
	callExternalHub(target, "SendToChannel", channel, hubMessage{Type: typeEvent, Data: data})
}

func callExternalHub(hub any, method string, channel string, msg hubMessage) {
	defer func() {
		if recovered := recover(); recovered != nil {
			return
		}
	}()

	methodValue := reflect.ValueOf(hub).MethodByName(method)
	if !methodValue.IsValid() {
		return
	}
	methodType := methodValue.Type()

	switch method {
	case "Broadcast":
		if methodType.NumIn() != 1 {
			return
		}
		arg, ok := hubMessageValue(methodType.In(0), msg)
		if !ok {
			return
		}
		methodValue.Call([]reflect.Value{arg})
	case "SendToChannel":
		if methodType.NumIn() != 2 {
			return
		}
		channelArg := reflect.ValueOf(channel)
		if !channelArg.Type().AssignableTo(methodType.In(0)) {
			if !channelArg.Type().ConvertibleTo(methodType.In(0)) {
				return
			}
			channelArg = channelArg.Convert(methodType.In(0))
		}
		messageArg, ok := hubMessageValue(methodType.In(1), msg)
		if !ok {
			return
		}
		methodValue.Call([]reflect.Value{channelArg, messageArg})
	}
}

func hubMessageValue(target reflect.Type, msg hubMessage) (reflect.Value, bool) {
	source := reflect.TypeOf(hubMessage{})
	value := reflect.ValueOf(msg)
	if value.Type().AssignableTo(target) {
		return value, true
	}
	if source.ConvertibleTo(target) {
		return value.Convert(target), true
	}
	if target.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	converted := reflect.New(target).Elem()
	setStringField(converted, "Type", string(msg.Type))
	setStringField(converted, "Channel", msg.Channel)
	setStringField(converted, "ProcessID", msg.ProcessID)
	setAnyField(converted, "Data", msg.Data)
	setTimeField(converted, "Timestamp", msg.Timestamp)
	return converted, true
}

func setStringField(target reflect.Value, name string, value string) {
	field := target.FieldByName(name)
	if !field.IsValid() || !field.CanSet() {
		return
	}
	if field.Kind() == reflect.String {
		field.SetString(value)
	}
}

func setAnyField(target reflect.Value, name string, value any) {
	field := target.FieldByName(name)
	if !field.IsValid() || !field.CanSet() || value == nil {
		return
	}
	raw := reflect.ValueOf(value)
	if raw.Type().AssignableTo(field.Type()) {
		field.Set(raw)
		return
	}
	if raw.Type().ConvertibleTo(field.Type()) {
		field.Set(raw.Convert(field.Type()))
	}
}

func setTimeField(target reflect.Value, name string, value time.Time) {
	field := target.FieldByName(name)
	if !field.IsValid() || !field.CanSet() {
		return
	}
	raw := reflect.ValueOf(value)
	if raw.Type().AssignableTo(field.Type()) {
		field.Set(raw)
	}
}
