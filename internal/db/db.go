package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
	"wow-guild-tracker/internal/models"
)

var DB *sql.DB

// InitDB инициализирует подключение к PostgreSQL и создаёт необходимые таблицы
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

	// Таблица для хранения привязки Battle.net
	_, err = DB.Exec(`
        CREATE TABLE IF NOT EXISTS battlenet_links (
            user_id TEXT PRIMARY KEY,
            linked BOOLEAN NOT NULL DEFAULT FALSE
        )
    `)
	if err != nil {
		return fmt.Errorf("failed to create battlenet_links table: %v", err)
	}

	log.Println("Database initialized successfully")
	return nil
}

// SaveCharacter сохраняет персонажа в базу
func SaveCharacter(userID, name, realm string, level int, class, guild string, mythicScore float64) error {
	var guildValue interface{}
	if guild == "" {
		guildValue = nil // Если guild пустой, сохраняем как NULL
	} else {
		guildValue = guild
	}

	_, err := DB.Exec(`
        INSERT INTO characters (user_id, name, realm, level, class, guild, mythic_score)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (user_id, name, realm)
        DO UPDATE SET level = $4, class = $5, guild = $6, mythic_score = $7
    `, userID, name, realm, level, class, guildValue, mythicScore)
	return err
}

// GetCharacters возвращает всех персонажей пользователя
func GetCharacters(userID string) ([]models.Character, error) {
	rows, err := DB.Query(`
        SELECT name, realm, level, class, guild, mythic_score 
        FROM characters 
        WHERE user_id = $1
    `, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query characters: %v", err)
	}
	defer rows.Close()

	var characters []models.Character
	for rows.Next() {
		var char models.Character
		var guild sql.NullString // Используем sql.NullString для обработки NULL

		err := rows.Scan(&char.Name, &char.Realm, &char.Level, &char.PlayableClass, &guild, &char.MythicScore)
		if err != nil {
			return nil, fmt.Errorf("failed to scan character: %v", err)
		}

		// Преобразуем sql.NullString в string
		if guild.Valid {
			char.Guild = guild.String
		} else {
			char.Guild = "" // Если NULL, устанавливаем пустую строку
		}

		characters = append(characters, char)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over characters: %v", err)
	}

	return characters, nil
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

// UnlinkDiscord удаляет привязку Discord
func UnlinkDiscord(userID string) error {
	_, err := DB.Exec(`
        DELETE FROM discord_links WHERE user_id = $1
    `, userID)
	return err
}

// SaveMainCharacter сохраняет главного персонажа с транзакцией
func SaveMainCharacter(userID, characterName, realm string) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	// Начинаем транзакцию
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}

	// Удаляем текущую запись о главном персонаже (если есть)
	_, err = tx.Exec("DELETE FROM main_characters WHERE user_id = $1", userID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete existing main character: %v", err)
	}

	// Устанавливаем нового главного персонажа
	_, err = tx.Exec(`
        INSERT INTO main_characters (user_id, character_name, realm)
        VALUES ($1, $2, $3)
    `, userID, characterName, realm)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to insert main character: %v", err)
	}

	// Подтверждаем транзакцию
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

// GetMainCharacter получает данные о главном персонаже
func GetMainCharacter(userID string) (characterName, realm string, err error) {
	row := DB.QueryRow(`
        SELECT character_name, realm FROM main_characters WHERE user_id = $1
    `, userID)
	err = row.Scan(&characterName, &realm)
	if err == sql.ErrNoRows {
		return "", "", nil // Нет главного персонажа
	}
	return
}

// LinkBattleNet привязывает Battle.net аккаунт
func LinkBattleNet(userID string) error {
	_, err := DB.Exec(`
        INSERT INTO battlenet_links (user_id, linked)
        VALUES ($1, TRUE)
        ON CONFLICT (user_id)
        DO UPDATE SET linked = TRUE
    `, userID)
	return err
}

// UnlinkBattleNet отвязывает Battle.net аккаунт и очищает связанные данные
func UnlinkBattleNet(userID string) error {
	// Начинаем транзакцию
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}

	// Удаляем персонажей
	_, err = tx.Exec(`
        DELETE FROM characters WHERE user_id = $1
    `, userID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete characters: %v", err)
	}

	// Удаляем главного персонажа
	_, err = tx.Exec(`
        DELETE FROM main_characters WHERE user_id = $1
    `, userID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete main character: %v", err)
	}

	// Удаляем привязку Battle.net
	_, err = tx.Exec(`
        DELETE FROM battlenet_links WHERE user_id = $1
    `, userID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete battlenet link: %v", err)
	}

	// Коммитим транзакцию
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

// HasBattleNetLink проверяет, привязан ли Battle.net
func HasBattleNetLink(userID string) bool {
	var linked bool
	err := DB.QueryRow(`
        SELECT linked FROM battlenet_links WHERE user_id = $1
    `, userID).Scan(&linked)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		log.Printf("Failed to check Battle.net link for user %s: %v", userID, err)
		return false
	}
	return linked
}

// GetDB возвращает подключение к базе данных (добавляем для совместимости)
func GetDB() (*sql.DB, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return DB, nil
}
