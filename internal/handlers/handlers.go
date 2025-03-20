package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"unicode"

	"golang.org/x/oauth2"
	"wow-guild-tracker/internal/api"
	"wow-guild-tracker/internal/db"
)

var (
	blizzardClientID     = os.Getenv("BLIZZARD_CLIENT_ID")
	blizzardClientSecret = os.Getenv("BLIZZARD_CLIENT_SECRET")
	discordClientID      = os.Getenv("DISCORD_CLIENT_ID")
	discordClientSecret  = os.Getenv("DISCORD_CLIENT_SECRET")

	// Маппинг английских названий реалмов на русские
	realmTranslations = map[string]string{
		"gordunni":      "Гордунни",
		"howling-fjord": "Ревущий Фьорд",
		"blackscar":     "Чёрный Шрам",
		"soulflayer":    "Свежеватель Душ",
	}

	// Конфигурация OAuth2 для Discord
	discordOauthConfig = &oauth2.Config{
		RedirectURL:  "http://localhost:8080/discord-callback",
		ClientID:     discordClientID,
		ClientSecret: discordClientSecret,
		Scopes:       []string{"identify"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://discord.com/api/oauth2/authorize",
			TokenURL: "https://discord.com/api/oauth2/token",
		},
	}
)

// generateState генерирует случайный state для OAuth2
func generateState() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func translateAndCapitalizeRealm(realm string) string {
	// Переводим реалм на русский, если он есть в маппинге
	translatedRealm, exists := realmTranslations[strings.ToLower(realm)]
	if !exists {
		translatedRealm = realm // Если реалм не найден, оставляем как есть
	}

	// Убеждаемся, что строка в UTF-8
	if len(translatedRealm) > 0 {
		runes := []rune(translatedRealm) // Работаем с рунами для корректной обработки Unicode
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
			return string(runes)
		}
	}
	return translatedRealm
}

// IndexHandler отображает главную страницу
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := r.URL.Query().Get("access_token")
	log.Printf("IndexHandler: access_token=%s", accessToken)
	if accessToken == "" {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		tmpl := template.Must(template.New("index.html").ParseFiles("templates/index.html"))
		if err := tmpl.Execute(w, nil); err != nil {
			log.Printf("Failed to render template (no token): %v", err)
			http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Получаем BattleTag пользователя
	battleTag := "Неизвестный пользователь"
	bt, err := api.FetchBattleTag(accessToken)
	if err != nil {
		log.Printf("Failed to fetch BattleTag: %v", err)
	} else {
		battleTag = bt
	}

	// Получаем данные о персонажах через Blizzard API
	chars, err := api.FetchAccountCharacters(accessToken)
	if err != nil {
		log.Printf("Failed to fetch characters: %v", err)
		http.Error(w, "Failed to fetch characters: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Сохраняем данные в базу (используем accessToken как временный userID)
	userID := accessToken
	for _, char := range chars.Characters {
		log.Printf("Saving character: Name=%s, Realm=%s, Level=%d, Class=%s, Guild=%s, MythicScore=%.1f, Role=%s, Spec=%s",
			char.Name, char.Realm, char.Level, char.PlayableClass, char.Guild, char.MythicScore, char.Role, char.Spec)
		err := db.SaveCharacter(
			userID,
			char.Name,
			char.Realm,
			char.Level,
			char.PlayableClass,
			char.Guild,
			char.MythicScore,
		)
		if err != nil {
			log.Printf("Failed to save character %s: %v", char.Name, err)
		}
	}

	// Получаем данные о главном персонаже
	mainCharName, mainCharRealm, err := db.GetMainCharacter(userID)
	if err != nil {
		log.Printf("Failed to get main character: %v", err)
	}

	// Переводим реалм главного персонажа
	translatedMainCharRealm := translateAndCapitalizeRealm(mainCharRealm)
	log.Printf("IndexHandler: MainCharName=%s, MainCharRealm=%s, TranslatedMainCharRealm=%s",
		mainCharName, mainCharRealm, translatedMainCharRealm)

	// Получаем данные о привязанном Discord
	discordID, discordName, err := db.GetDiscordLink(userID)
	if err != nil {
		log.Printf("Failed to get Discord link: %v", err)
	}

	// Обрезаем #0 из discordName
	if discordName != "" {
		parts := strings.Split(discordName, "#")
		if len(parts) > 0 {
			discordName = parts[0]
		}
	}

	// Проверяем привязку Battle.net по наличию accessToken
	hasBattleNetLink := accessToken != ""

	// Создаём новый срез с переведёнными реалмами и статусом "Main"
	type DisplayCharacter struct {
		Name            string
		Realm           string
		Level           int
		PlayableClass   string
		Guild           string
		MythicScore     float64
		TranslatedRealm string
		IsMain          bool
		Role            string // Оставляем для отображения
		Spec            string // Оставляем для отображения
		RoleIcon        string // Название файла иконки для роли
		SpecIcon        string // Название файла иконки для специализации
	}
	var displayCharacters []DisplayCharacter
	for _, char := range chars.Characters {
		translatedRealm := translateAndCapitalizeRealm(char.Realm)
		isMain := (char.Name == mainCharName && char.Realm == mainCharRealm)

		// Подготавливаем названия иконок
		roleIcon := "Unknown"
		if char.Role != "" && char.Role != "Unknown" {
			roleIcon = strings.ToLower(strings.ReplaceAll(char.Role, " ", "_"))
		}
		specIcon := "Unknown"
		if char.Spec != "" && char.Spec != "Unknown" {
			specIcon = strings.ToLower(strings.ReplaceAll(char.Spec, " ", "_"))
		}

		displayChar := DisplayCharacter{
			Name:            char.Name,
			Realm:           char.Realm,
			Level:           char.Level,
			PlayableClass:   char.PlayableClass,
			Guild:           char.Guild,
			MythicScore:     char.MythicScore,
			TranslatedRealm: translatedRealm,
			IsMain:          isMain,
			Role:            char.Role,
			Spec:            char.Spec,
			RoleIcon:        roleIcon,
			SpecIcon:        specIcon,
		}
		displayCharacters = append(displayCharacters, displayChar)
		log.Printf("IndexHandler: Character=%s, TranslatedRealm=%s, IsMain=%v, Role=%s, Spec=%s, RoleIcon=%s, SpecIcon=%s",
			char.Name, translatedRealm, isMain, char.Role, char.Spec, roleIcon, specIcon)
	}

	// Передаём данные в шаблон
	data := struct {
		Characters       []DisplayCharacter
		AccessToken      string
		BattleTag        string
		UserID           string
		MainCharName     string
		MainCharRealm    string
		DiscordID        string
		DiscordName      string
		HasDiscordLink   bool
		HasBattleNetLink bool
	}{
		Characters:       displayCharacters,
		AccessToken:      accessToken,
		BattleTag:        battleTag,
		UserID:           userID,
		MainCharName:     mainCharName,
		MainCharRealm:    translatedMainCharRealm,
		DiscordID:        discordID,
		DiscordName:      discordName,
		HasDiscordLink:   discordID != "",
		HasBattleNetLink: hasBattleNetLink,
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	tmpl := template.Must(template.New("index.html").ParseFiles("templates/index.html"))
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Failed to render template: %v", err)
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}

// LoginHandler перенаправляет на Blizzard OAuth
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Генерируем state
	state, err := generateState()
	if err != nil {
		log.Printf("LoginHandler: Failed to generate state: %v", err)
		http.Error(w, "Failed to generate state: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Сохраняем state в куки
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
	})

	// Формируем URL с параметром state, добавляем scope openid
	url := fmt.Sprintf(
		"https://eu.battle.net/oauth/authorize?client_id=%s&redirect_uri=http://localhost:8080/callback&response_type=code&scope=wow.profile+openid&state=%s",
		blizzardClientID, state,
	)
	log.Printf("LoginHandler: Redirecting to Blizzard OAuth URL: %s", url)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// CallbackHandler обрабатывает обратный вызов от Blizzard
func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем state
	cookie, err := r.Cookie("oauth_state")
	if err != nil {
		log.Printf("CallbackHandler: State cookie not found: %v", err)
		http.Error(w, "State cookie not found", http.StatusBadRequest)
		return
	}

	returnedState := r.URL.Query().Get("state")
	if returnedState == "" || returnedState != cookie.Value {
		log.Printf("CallbackHandler: Invalid state parameter: returned=%s, expected=%s", returnedState, cookie.Value)
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Очищаем куки
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	// Получаем код
	code := r.URL.Query().Get("code")
	if code == "" {
		log.Printf("CallbackHandler: No code provided")
		http.Error(w, "No code provided", http.StatusBadRequest)
		return
	}

	// Обмениваем код на токен
	tokenURL := "https://eu.battle.net/oauth/token"
	client := &http.Client{}
	data := strings.NewReader(fmt.Sprintf(
		"grant_type=authorization_code&code=%s&client_id=%s&client_secret=%s&redirect_uri=http://localhost:8080/callback",
		code, blizzardClientID, blizzardClientSecret,
	))
	req, err := http.NewRequest("POST", tokenURL, data)
	if err != nil {
		log.Printf("CallbackHandler: Failed to create token request: %v", err)
		http.Error(w, "Failed to create token request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("CallbackHandler: Failed to exchange code for token: %v", err)
		http.Error(w, "Failed to exchange code for token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		log.Printf("CallbackHandler: Failed to parse token response: %v", err)
		http.Error(w, "Failed to parse token response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("CallbackHandler: Successfully received access_token: %s", tokenResp.AccessToken)
	// Перенаправляем на главную страницу с токеном
	http.Redirect(w, r, "/?access_token="+tokenResp.AccessToken, http.StatusSeeOther)
}

// LinkDiscordHandler перенаправляет на Discord OAuth
func LinkDiscordHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := r.URL.Query().Get("access_token")
	if accessToken == "" {
		log.Printf("LinkDiscordHandler: No access token provided")
		http.Error(w, "Access token is required", http.StatusBadRequest)
		return
	}

	// Генерируем state для Discord
	state, err := generateState()
	if err != nil {
		log.Printf("LinkDiscordHandler: Failed to generate state: %v", err)
		http.Error(w, "Failed to generate state: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Сохраняем state и accessToken в куки
	http.SetCookie(w, &http.Cookie{
		Name:     "discord_oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
	})

	// Перенаправляем на Discord OAuth
	url := discordOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
	log.Printf("LinkDiscordHandler: Redirecting to Discord OAuth URL: %s", url)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// DiscordCallbackHandler обрабатывает обратный вызов от Discord
func DiscordCallbackHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем state
	cookie, err := r.Cookie("discord_oauth_state")
	if err != nil {
		log.Printf("DiscordCallbackHandler: Discord state cookie not found: %v", err)
		http.Error(w, "Discord state cookie not found", http.StatusBadRequest)
		return
	}

	returnedState := r.URL.Query().Get("state")
	if returnedState == "" || returnedState != cookie.Value {
		log.Printf("DiscordCallbackHandler: Invalid Discord state parameter: returned=%s, expected=%s", returnedState, cookie.Value)
		http.Error(w, "Invalid Discord state parameter", http.StatusBadRequest)
		return
	}

	// Получаем accessToken из куки
	accessTokenCookie, err := r.Cookie("access_token")
	if err != nil {
		log.Printf("DiscordCallbackHandler: Access token cookie not found: %v", err)
		http.Error(w, "Access token cookie not found", http.StatusBadRequest)
		return
	}
	accessToken := accessTokenCookie.Value

	// Очищаем куки
	http.SetCookie(w, &http.Cookie{
		Name:     "discord_oauth_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	// Получаем код
	code := r.URL.Query().Get("code")
	if code == "" {
		log.Printf("DiscordCallbackHandler: No code provided")
		http.Error(w, "No code provided", http.StatusBadRequest)
		return
	}

	// Обмениваем код на токен
	token, err := discordOauthConfig.Exchange(r.Context(), code)
	if err != nil {
		log.Printf("DiscordCallbackHandler: Failed to exchange code for token: %v", err)
		http.Error(w, "Failed to exchange code for token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Запрашиваем данные пользователя Discord
	client := discordOauthConfig.Client(r.Context(), token)
	resp, err := client.Get("https://discord.com/api/users/@me")
	if err != nil {
		log.Printf("DiscordCallbackHandler: Failed to fetch Discord user info: %v", err)
		http.Error(w, "Failed to fetch Discord user info: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var user struct {
		ID            string `json:"id"`
		Username      string `json:"username"`
		Discriminator string `json:"discriminator"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		log.Printf("DiscordCallbackHandler: Failed to parse Discord user info: %v", err)
		http.Error(w, "Failed to parse Discord user info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Формируем никнейм в формате username#discriminator
	discordName := fmt.Sprintf("%s#%s", user.Username, user.Discriminator)

	// Сохраняем данные в базе, используем accessToken как userID
	userID := accessToken
	if err := db.SaveDiscordLink(userID, user.ID, discordName); err != nil {
		log.Printf("DiscordCallbackHandler: Failed to save Discord link: %v", err)
		http.Error(w, "Failed to save Discord link: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Перенаправляем обратно на главную страницу
	log.Printf("DiscordCallbackHandler: Redirecting to /?access_token=%s", accessToken)
	http.Redirect(w, r, "/?access_token="+accessToken, http.StatusSeeOther)
}

func SetMainHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("SetMainHandler: Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	characterName := r.FormValue("character_name")
	realm := r.FormValue("realm")
	accessToken := r.FormValue("access_token")
	log.Printf("SetMainHandler: Received POST - character_name=%s, realm=%s, access_token=%s", characterName, realm, accessToken)

	if characterName == "" || realm == "" || accessToken == "" {
		log.Printf("SetMainHandler: Missing required fields - character_name=%s, realm=%s, access_token=%s", characterName, realm, accessToken)
		http.Error(w, "Character name, realm, and access token are required", http.StatusBadRequest)
		return
	}

	// Используем accessToken как временный userID
	userID := accessToken
	err := db.SaveMainCharacter(userID, characterName, realm)
	if err != nil {
		log.Printf("SetMainHandler: Failed to set main character: %v", err)
		http.Error(w, "Failed to set main character: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("SetMainHandler: Successfully set main character: %s, %s for user %s", characterName, realm, userID)
	// Перенаправляем обратно на главную страницу
	http.Redirect(w, r, "/?access_token="+accessToken, http.StatusSeeOther)
}

// UnlinkBattleNetHandler отвязывает Battle.net аккаунт
func UnlinkBattleNetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := "some_user_id" // Замените на реальный userID при интеграции
	log.Printf("UnlinkBattleNetHandler: user_id=%s", userID)

	err := db.UnlinkBattleNet(userID)
	if err != nil {
		log.Printf("Failed to unlink Battle.net for user %s: %v", userID, err)
		http.Error(w, "Failed to unlink Battle.net: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Перенаправляем на главную страницу
	accessToken := r.URL.Query().Get("access_token")
	http.Redirect(w, r, "/?access_token="+accessToken, http.StatusSeeOther)
}

// UnlinkDiscordHandler отвязывает Discord аккаунт
func UnlinkDiscordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := "some_user_id" // Замените на реальный userID при интеграции
	log.Printf("UnlinkDiscordHandler: user_id=%s", userID)

	err := db.UnlinkDiscord(userID)
	if err != nil {
		log.Printf("Failed to unlink Discord for user %s: %v", userID, err)
		http.Error(w, "Failed to unlink Discord: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Перенаправляем на главную страницу
	accessToken := r.URL.Query().Get("access_token")
	http.Redirect(w, r, "/?access_token="+accessToken, http.StatusSeeOther)
}

// LogoutHandler выполняет выход из аккаунта
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := "some_user_id" // Замените на реальный userID при интеграции
	log.Printf("LogoutHandler: user_id=%s", userID)

	// Очистка всех данных пользователя
	err := db.UnlinkBattleNet(userID)
	if err != nil {
		log.Printf("Failed to unlink Battle.net during logout for user %s: %v", userID, err)
	}
	err = db.UnlinkDiscord(userID)
	if err != nil {
		log.Printf("Failed to unlink Discord during logout for user %s: %v", userID, err)
	}

	// Перенаправляем на главную страницу
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
