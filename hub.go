package main

import (
	"log"
	"time"
)

type Message struct {
	data []byte
	room string
}

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	roomID      string
	discription string
	time        int64

	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func newHub(roomID, discription string) *Hub {
	// func newHub() *Hub {
	return &Hub{
		roomID:      roomID,
		discription: discription,
		time:        time.Now().Unix(),
		broadcast:   make(chan []byte),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		clients:     make(map[*Client]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true

			// connections := h.rooms[client.room]
			// if connections == nil {
			// 	connections = make(map[*Client]bool)
			// 	h.rooms[client.room] = connections
			// }
			// connections[client.conn] = true

		case client := <-h.unregister:
			log.Println("unregister")
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
