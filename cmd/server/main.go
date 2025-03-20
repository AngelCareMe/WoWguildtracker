package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"wow-guild-tracker/internal/db"
	"wow-guild-tracker/internal/handlers"
)

func main() {
	// Инициализация базы данных
	err := db.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Настройка маршрутов
	http.HandleFunc("/", handlers.IndexHandler)
	http.HandleFunc("/login", handlers.LoginHandler)
	http.HandleFunc("/callback", handlers.CallbackHandler)
	http.HandleFunc("/unlink-battlenet", handlers.UnlinkBattleNetHandler)
	http.HandleFunc("/unlink-discord", handlers.UnlinkDiscordHandler)
	http.HandleFunc("/logout", handlers.LogoutHandler)
	http.HandleFunc("/link-discord", handlers.LinkDiscordHandler)
	http.HandleFunc("/discord-callback", handlers.DiscordCallbackHandler)
	http.HandleFunc("/set-main", handlers.SetMainHandler)

	// Явная обработка статических файлов
	http.HandleFunc("/static/", serveStaticFiles)

	// Запуск сервера
	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// serveStaticFiles обрабатывает запросы к статическим файлам
func serveStaticFiles(w http.ResponseWriter, r *http.Request) {
	// Удаляем префикс /static/ из пути
	filePath := r.URL.Path[len("/static/"):]

	// Абсолютный путь к файлу в проекте
	staticDir := http.Dir("static")
	file := filepath.Join(string(staticDir), filePath)

	// Открываем файл
	f, err := os.Open(file)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		log.Printf("Error opening file %s: %v", file, err)
		return
	}
	defer f.Close()

	// Определяем MIME-тип (явно задаём для CSS)
	var contentType string
	if filepath.Ext(file) == ".css" {
		contentType = "text/css; charset=utf-8"
	} else if filepath.Ext(file) == ".jpg" || filepath.Ext(file) == ".png" || filepath.Ext(file) == ".gif" {
		contentType = "image/" + filepath.Ext(file)[1:]
	} else {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)

	// Копируем содержимое файла в ответ
	_, err = io.Copy(w, f)
	if err != nil {
		http.Error(w, "Error serving file", http.StatusInternalServerError)
		log.Printf("Error serving file %s: %v", file, err)
	}
}
