package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
)

var addr = flag.String("addr", ":8080", "http service address")

type roomJSON struct {
	RoomName    string `json:"roomName"`
	Discription string `json:"discription"`
}

var Hubs = []*Hub{}

func createRoom(w http.ResponseWriter, r *http.Request) {
	body := r.Body
	defer body.Close()
	buf := new(bytes.Buffer)
	io.Copy(buf, body)
	var roomJson roomJSON
	json.Unmarshal(buf.Bytes(), &roomJson)

	id, err := uuid.NewRandom()
	if err != nil {
		return
	}

	go func() {
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, time.Second*200)
		defer cancel()

		hub := newHub(id.String(), roomJson.RoomName, roomJson.Discription)
		go hub.run(ctx)

		Hubs = append(Hubs, hub)

		select {
		case <-ctx.Done():
			fmt.Println("done1:", ctx.Err())
			for i, v := range Hubs {
				if v.RoomID == id.String() {
					Hubs = append(Hubs[:i], Hubs[i+1:]...)
				}
			}
		}
	}()
}

func getRooms(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, Hubs)
}

func getSessionID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.NewRandom()
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, id)
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
	if origin == "http://localhost:4200" || origin == "https://fir-angular-showcase.web.app" {
		return true
	}
	return false
}

func RoomCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roomID := chi.URLParam(r, "roomID")

		for _, v := range Hubs {
			if v.RoomID == roomID {
				ctx := context.WithValue(r.Context(), "roomID", v)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		http.Error(w, http.StatusText(404), 404)
		return
	})
}

func UserCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userIDValues := r.URL.Query()
		if len(userIDValues["userID"]) > 0 {
			userID := userIDValues["userID"][0]
			if len(userID) > 0 {
				ctx := context.WithValue(r.Context(), "userID", userID)
				next.ServeHTTP(w, r.WithContext(ctx))
			} else {
				http.Error(w, http.StatusText(400), 400)
				return
			}
		} else {
			http.Error(w, http.StatusText(404), 404)
			return
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

	r.Get("/rooms", getRooms)
	r.Get("/sessionid", getSessionID)

	r.Route("/ws", func(r chi.Router) {
		r.Route("/{roomID}", func(r chi.Router) {
			r.Use(RoomCtx)
			r.Use(UserCtx)
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				hub, ok := ctx.Value("roomID").(*Hub)
				if !ok {
					http.Error(w, http.StatusText(422), 422)
					return
				}
				user := ctx.Value("userID").(string)
				serveWs(hub, user, w, r)
			})
		})
	})

	err := http.ListenAndServe(*addr, r)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
