package main

import (
	"log"
	"net/http"
	"url_shortener/internal/handler"
	"url_shortener/internal/store"
)

func main() {
	mux := http.NewServeMux()
	store := store.NewInMemoryStore()

	log.Println("Server started on port 8080")
	mux.HandleFunc("/shorten", handler.ShortenHandler(store))
	mux.HandleFunc("/", handler.RedirectHandler(store))
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}
