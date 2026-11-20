// Command server runs the API.
package main

import (
	"log"
	"net/http"
	"os"

	"example.com/authsvc/internal/api"
	"example.com/authsvc/internal/auth"
	"example.com/authsvc/internal/store"
)

func main() {
	db, err := store.Open(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	svc := auth.NewService(db, auth.SteamProvider{Key: os.Getenv("STEAM_KEY")}, auth.GoogleProvider{})
	h := &api.LoginHandler{Auth: svc}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", h.ServeLogin)
	log.Fatal(http.ListenAndServe(":8080", mux))
}
