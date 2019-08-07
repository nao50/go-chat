package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
)

var addr = flag.String("addr", ":8080", "http service address")

type roomJSON struct {
	Name        string `json:"name"`
	Discription string `json:"discription"`
}

var Hubs = map[string]*Hub{}

func createRoom(w http.ResponseWriter, r *http.Request) {
	body := r.Body
	defer body.Close()

	buf := new(bytes.Buffer)
	io.Copy(buf, body)
	var roomJson roomJSON
	json.Unmarshal(buf.Bytes(), &roomJson)

	hub := newHub(roomJson.Name, roomJson.Discription)
	go hub.run()

	Hubs[roomJson.Name] = hub

	log.Printf("Hubs: %+v\n", Hubs)
	log.Printf("hub: %+v\n", hub)
}

func getRooms(w http.ResponseWriter, r *http.Request) {
	if len(Hubs) > 0 {
		respondJSON(w, http.StatusOK, keys(Hubs))
	} else {
		respondJSON(w, http.StatusOK, "No chat room exist")
	}
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "home.html")
}

func AllowOriginFunc(r *http.Request, origin string) bool {
	// if origin == "http://localhost:4200" || origin == "https://fir-angular-showcase.web.app" {
	// 	return true
	// }
	// return false
	return true
}

func RoomCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roomID := chi.URLParam(r, "roomID")

		if value, ok := Hubs[roomID]; !ok {
			http.Error(w, http.StatusText(404), 404)
			return
		} else {
			ctx := context.WithValue(r.Context(), "hub", value)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	response, err := json.MarshalIndent(payload, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(response))
}

func keys(m map[string]*Hub) []string {
	ks := []string{}
	for k, _ := range m {
		ks = append(ks, k)
	}
	return ks
}

func main() {
	flag.Parse()

	r := chi.NewRouter()

	cors := cors.New(cors.Options{
		AllowOriginFunc:  AllowOriginFunc,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	})

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler)

	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/", serveHome)
	r.Post("/createroom", createRoom)

	r.Get("/getRooms", getRooms)

	r.Route("/ws", func(r chi.Router) {
		r.Route("/{roomID}", func(r chi.Router) {
			r.Use(RoomCtx)
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				hub, ok := ctx.Value("hub").(*Hub)
				if !ok {
					http.Error(w, http.StatusText(422), 422)
					return
				}
				serveWs(hub, w, r)
			})
		})
	})

	err := http.ListenAndServe(*addr, r)

	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
