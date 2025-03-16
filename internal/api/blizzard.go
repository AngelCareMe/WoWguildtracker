package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"wow-guild-tracker/internal/models"
)

// FetchAccountCharacters запрашивает данные о персонажах аккаунта из Blizzard API
func FetchAccountCharacters(accessToken string) (*models.AccountCharacters, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("access token is empty")
	}

	url := "https://eu.api.blizzard.com/profile/user/wow?namespace=profile-eu&locale=ru_Ru"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Failed to create Blizzard API request: %v", err)
		return nil, fmt.Errorf("failed to create Blizzard API request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch account characters: %v", err)
		return nil, fmt.Errorf("failed to fetch account characters: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Blizzard API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("Blizzard API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var accountData struct {
		WowAccounts []struct {
			Characters []struct {
				Name          string `json:"name"`
				Level         int    `json:"level"`
				PlayableClass struct {
					Name string `json:"name"`
				} `json:"playable_class"`
				Realm struct {
					Slug string `json:"slug"`
				} `json:"realm"`
				Guild struct {
					Name string `json:"name"`
				} `json:"guild"`
			} `json:"characters"`
		} `json:"wow_accounts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&accountData); err != nil {
		log.Printf("Failed to parse Blizzard API response: %v", err)
		return nil, fmt.Errorf("failed to parse Blizzard API response: %v", err)
	}

	var characters []models.Character
	for _, account := range accountData.WowAccounts {
		for _, char := range account.Characters {
			normalizedName := strings.ToLower(strings.TrimSpace(char.Name))
			normalizedRealm := strings.ToLower(strings.TrimSpace(char.Realm.Slug))

			character := models.Character{
				Name:          char.Name,
				Realm:         char.Realm.Slug,
				Level:         char.Level,
				PlayableClass: char.PlayableClass.Name,
				Guild:         char.Guild.Name, // Используем значение из API, если оно есть
			}

			guildName, err := fetchCharacterProfile(normalizedName, normalizedRealm, accessToken)
			if err != nil {
				log.Printf("Failed to fetch character profile for %s on %s: %v", char.Name, char.Realm.Slug, err)
				// Не устанавливаем "N/A", оставляем пустую строку
			} else if guildName == "" {
				log.Printf("No guild found for character %s on %s", char.Name, char.Realm.Slug)
				// Оставляем Guild пустым
			} else {
				character.Guild = guildName
			}

			mythicScore, err := fetchMythicKeystoneProfile(normalizedName, normalizedRealm, accessToken)
			if err != nil {
				log.Printf("Failed to fetch Mythic+ score for %s on %s: %v, response: %v", char.Name, char.Realm.Slug, err, mythicScore)
				character.MythicScore = 0.0
			} else {
				character.MythicScore = mythicScore
				log.Printf("Fetched Mythic+ score for %s on %s: %.1f", char.Name, char.Realm.Slug, mythicScore)
			}

			characters = append(characters, character)
		}
	}

	return &models.AccountCharacters{
		Characters: characters,
	}, nil
}

// fetchCharacterProfile запрашивает профиль персонажа для получения данных о гильдии
func fetchCharacterProfile(name, realm, accessToken string) (string, error) {
	url := fmt.Sprintf("https://eu.api.blizzard.com/profile/wow/character/%s/%s?namespace=profile-eu&locale=ru_RU",
		realm, name)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Failed to create character profile request for %s on %s: %v", name, realm, err)
		return "", fmt.Errorf("failed to create character profile request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch character profile for %s on %s: %v", name, realm, err)
		return "", fmt.Errorf("failed to fetch character profile: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		log.Printf("Character profile not found for %s on %s", name, realm)
		return "", nil // Возвращаем пустую строку
	} else if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Character profile API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
		return "", fmt.Errorf("Character profile API returned status: %d", resp.StatusCode)
	}

	var charData struct {
		Guild struct {
			Name string `json:"name"`
		} `json:"guild"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&charData); err != nil {
		log.Printf("Failed to parse character profile response for %s on %s: %v", name, realm, err)
		return "", fmt.Errorf("failed to parse character profile response: %v", err)
	}

	return charData.Guild.Name, nil
}

// fetchMythicKeystoneProfile запрашивает Mythic+ рейтинг из Blizzard API
func fetchMythicKeystoneProfile(name, realm, accessToken string) (float64, error) {
	url := fmt.Sprintf("https://eu.api.blizzard.com/profile/wow/character/%s/%s/mythic-keystone-profile?namespace=profile-eu&locale=ru_RU",
		realm, name)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Failed to create Mythic+ request for %s on %s: %v", name, realm, err)
		return 0.0, fmt.Errorf("failed to create Mythic+ request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch Mythic+ profile for %s on %s: %v", name, realm, err)
		return 0.0, fmt.Errorf("failed to fetch Mythic+ profile: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("Mythic+ response for %s on %s: status %d, body: %s", name, realm, resp.StatusCode, string(bodyBytes))

	if resp.StatusCode == http.StatusNotFound {
		log.Printf("Mythic+ profile not found for %s on %s", name, realm)
		return 0.0, nil
	} else if resp.StatusCode != http.StatusOK {
		log.Printf("Mythic+ API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
		return 0.0, fmt.Errorf("Mythic+ API returned status: %d", resp.StatusCode)
	}

	var mythicData struct {
		CurrentPeriod struct {
			Period struct {
				BestRuns []struct {
					MythicRating struct {
						Rating float64 `json:"rating"`
					} `json:"mythic_rating"`
				} `json:"best_runs"`
			} `json:"period"`
		} `json:"current_period"`
		CurrentMythicRating struct {
			Rating float64 `json:"rating"`
		} `json:"current_mythic_rating"`
	}
	if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&mythicData); err != nil {
		log.Printf("Failed to parse Mythic+ response for %s on %s: %v", name, realm, err)
		return 0.0, fmt.Errorf("failed to parse Mythic+ response: %v", err)
	}

	// Проверяем current_mythic_rating.rating в первую очередь
	if mythicData.CurrentMythicRating.Rating > 0 {
		log.Printf("Using current_mythic_rating for %s on %s: %.1f", name, realm, mythicData.CurrentMythicRating.Rating)
		return mythicData.CurrentMythicRating.Rating, nil
	}

	// Если current_mythic_rating пустой, ищем максимальный рейтинг в best_runs
	if len(mythicData.CurrentPeriod.Period.BestRuns) > 0 {
		maxRating := 0.0
		for _, run := range mythicData.CurrentPeriod.Period.BestRuns {
			if run.MythicRating.Rating > maxRating {
				maxRating = run.MythicRating.Rating
			}
		}
		if maxRating > 0 {
			log.Printf("Using max rating from best_runs for %s on %s: %.1f", name, realm, maxRating)
			return maxRating, nil
		}
	}

	log.Printf("No Mythic+ runs or current rating found for %s on %s", name, realm)
	return 0.0, nil
}
