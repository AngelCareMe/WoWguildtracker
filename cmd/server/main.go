package main

import (
	"log"
	"net/http"

	"wow-guild-tracker/internal/db"
	"wow-guild-tracker/internal/handlers"
)

func main() {
	if err := db.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Регистрация обработчиков
	http.HandleFunc("/", handlers.IndexHandler)
	http.HandleFunc("/login", handlers.LoginHandler)
	http.HandleFunc("/callback", handlers.CallbackHandler)
	http.HandleFunc("/link-discord", handlers.LinkDiscordHandler)
	http.HandleFunc("/discord-callback", handlers.DiscordCallbackHandler)
	http.HandleFunc("/set-main", handlers.SetMainHandler)

	// Обслуживание статических файлов (изображения и favicon)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
