package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	var err error
	DB, err = sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	err = DB.Ping()
	if err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	// Таблица для хранения данных о персонажах
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS characters (
			id SERIAL PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			realm TEXT NOT NULL,
			level INTEGER NOT NULL,
			class TEXT NOT NULL,
			guild TEXT,
			mythic_score FLOAT,
			UNIQUE(user_id, name, realm)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create characters table: %v", err)
	}

	// Таблица для привязки Discord
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS discord_links (
			user_id TEXT PRIMARY KEY,
			discord_id TEXT NOT NULL,
			discord_name TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create discord_links table: %v", err)
	}

	// Таблица для главного персонажа
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS main_characters (
			user_id TEXT PRIMARY KEY,
			character_name TEXT NOT NULL,
			realm TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create main_characters table: %v", err)
	}

	log.Println("Database initialized successfully")
	return nil
}

func SaveCharacter(userID, name, realm string, level int, class, guild string, mythicScore float64) error {
	_, err := DB.Exec(`
		INSERT INTO characters (user_id, name, realm, level, class, guild, mythic_score)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, name, realm)
		DO UPDATE SET level = $4, class = $5, guild = $6, mythic_score = $7
	`, userID, name, realm, level, class, guild, mythicScore)
	return err
}

// SaveDiscordLink сохраняет привязку Discord-аккаунта
func SaveDiscordLink(userID, discordID, discordName string) error {
	_, err := DB.Exec(`
		INSERT INTO discord_links (user_id, discord_id, discord_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id)
		DO UPDATE SET discord_id = $2, discord_name = $3
	`, userID, discordID, discordName)
	return err
}

// GetDiscordLink получает данные о привязанном Discord-аккаунте
func GetDiscordLink(userID string) (discordID, discordName string, err error) {
	row := DB.QueryRow(`
		SELECT discord_id, discord_name FROM discord_links WHERE user_id = $1
	`, userID)
	err = row.Scan(&discordID, &discordName)
	if err == sql.ErrNoRows {
		return "", "", nil // Возвращаем пустые значения, если записи нет
	}
	return
}

// SaveMainCharacter сохраняет главного персонажа
func SaveMainCharacter(userID, characterName, realm string) error {
	_, err := DB.Exec(`
		INSERT INTO main_characters (user_id, character_name, realm)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id)
		DO UPDATE SET character_name = $2, realm = $3
	`, userID, characterName, realm)
	return err
}

// GetMainCharacter получает данные о главном персонаже
func GetMainCharacter(userID string) (characterName, realm string, err error) {
	row := DB.QueryRow(`
		SELECT character_name, realm FROM main_characters WHERE user_id = $1
	`, userID)
	err = row.Scan(&characterName, &realm)
	if err == sql.ErrNoRows {
		return "", "", nil // Возвращаем пустые значения, если записи нет
	}
	return
}
