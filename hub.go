package main

import (
	"context"
	"time"
)

type Message struct {
	UserId string `json:"userId"`
	Time   int64  `json:"time"`
	RoomID string `json:"roomID"`
	Data   []byte `json:"data"`
}

type Hub struct {
	RoomID       string          `json:"roomID"`
	RoomName     string          `json:"roomName"`
	Discription  string          `json:"discription"`
	Time         int64           `json:"time"`
	Existclients map[string]bool `json:"existclients"`
	Messages     []Message       `json:"message"`

	// Registered clients.
	clients map[*Client]bool `json:"clients"`
	// Inbound messages from the clients.
	broadcast chan []byte `json:"broadcast"`
	// Register requests from the clients.
	register chan *Client `json:"register"`
	// Unregister requests from clients.
	unregister chan *Client `json:"unregister"`
}

func newHub(roomID, roomName, discription string) *Hub {
	return &Hub{
		RoomID:       roomID,
		RoomName:     roomName,
		Discription:  discription,
		Time:         time.Now().UnixNano() / int64(time.Millisecond),
		Existclients: make(map[string]bool),
		Messages:     []Message{},
		clients:      make(map[*Client]bool),
		broadcast:    make(chan []byte),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
	}
}

func (h *Hub) run(ctx context.Context) {
	hubCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			h.Existclients[client.UserId] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				h.Existclients[client.UserId] = false
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
					var m Message = Message{
						UserId: client.UserId,
						Time:   time.Now().UnixNano() / int64(time.Millisecond),
						RoomID: h.RoomID,
						Data:   message,
					}
					h.Messages = append(h.Messages, m)
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		case <-hubCtx.Done():
			for client := range h.clients {
				h.Existclients[client.UserId] = false
				delete(h.clients, client)
				close(client.send)
			}
			return
		}
	}
}
