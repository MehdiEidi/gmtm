package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
)

const (
	TELEGRAM_API_BASE_URL     = "https://api.telegram.org/bot"
	TELEGRAM_API_SEND_MESSAGE = "/sendMessage"
	BOT_TOKEN_ENV             = "TELEGRAM_BOT_TOKEN"
	IMDB_URL                  = "https://www.imdb.com/search/keyword/?keywords="
)

var telegramAPI = TELEGRAM_API_BASE_URL + os.Getenv(BOT_TOKEN_ENV) + TELEGRAM_API_SEND_MESSAGE

// Update is a Telegram object that we receive every time a user interacts with the bot.
type Update struct {
	UpdateID int     `json:"update_id"`
	Message  Message `json:"message"`
}

// String implements the fmt.String interface to get the representation of an Update as a string.
func (u Update) String() string {
	return fmt.Sprintf("(update id: %d, message: %s)", u.UpdateID, u.Message)
}

// Message is a Telegram object that can be found in an update.
type Message struct {
	Text     string   `json:"text"`
	Chat     Chat     `json:"chat"`
	Audio    Audio    `json:"audio"`
	Voice    Voice    `json:"voice"`
	Document Document `json:"document"`
}

// String implements the fmt.String interface to get the representation of a Message as a string.
func (m Message) String() string {
	return fmt.Sprintf("(text: %s, chat: %s, audio %s)", m.Text, m.Chat, m.Audio)
}

// Audio refer to a audio file sent.
type Audio struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
}

// String implements the fmt.String interface to get the representation of an Audio as a string.
func (a Audio) String() string {
	return fmt.Sprintf("(file id: %s, duration: %d)", a.FileID, a.Duration)
}

// Voice can be summarized with similar attribute as an Audio message for our use case.
type Voice Audio

// Document refer to a file sent.
type Document struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
}

// String implements the fmt.String interface to get the representation of an Document as a string.
func (d Document) String() string {
	return fmt.Sprintf("(file id: %s, file name: %s)", d.FileID, d.FileName)
}

// Chat indicates the conversation to which the Message belongs.
type Chat struct {
	ID int `json:"id"`
}

// String implements the fmt.String interface to get the representation of a Chat as a string.
func (c Chat) String() string {
	return fmt.Sprintf("(id: %d)", c.ID)
}

// Handler sends a message back to the chat.
func Handler(w http.ResponseWriter, r *http.Request) {
	update, err := parseIncomingRequest(r)
	if err != nil {
		log.Printf("error parsing incoming update, %s", err.Error())
		return
	}

	telegramResponseBody, err := sendToClient(update.Message.Chat.ID, strings.ToLower(update.Message.Text))
	if err != nil {
		log.Printf("got error %s from telegram, response body is %s", err.Error(), telegramResponseBody)
		return
	}

	log.Printf("successfully distributed to chat id %d", update.Message.Chat.ID)
}

// parseIncomingRequest parses incoming update to Update.
func parseIncomingRequest(r *http.Request) (*Update, error) {
	var update Update

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Printf("could not decode incoming update %s", err.Error())
		return nil, err
	}

	if update.UpdateID == 0 {
		log.Printf("invalid update id, got update id = 0")
		return nil, errors.New("invalid update id. 0 indicates failure to parse incoming update")
	}

	return &update, nil
}

// sendToClient sends a text message to the Telegram chat identified by the chat ID.
func sendToClient(chatID int, incomingText string) (string, error) {
	if incomingText == "/start" {
		response, err := http.PostForm(telegramAPI, url.Values{
			"chat_id": {strconv.Itoa(chatID)},
			"text":    {"Hey dude!\nGive me some keywords (comma delimited) to recommend you movies :D"},
		})
		if err != nil {
			log.Printf("error when posting text to the chat: %s", err.Error())
			return "", err
		}
		defer response.Body.Close()

		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Printf("error in parsing telegram response %s", err.Error())
			return "", err
		}

		log.Printf("body of the telegram response: %s", string(body))

		return string(body), nil
	}

	sendValues := url.Values{"chat_id": {strconv.Itoa(chatID)}}

	switch incomingText {
	case "start":
		text := "Hey dude!\nGive me some keywords (comma delimited) to recommend you movies :D"
		sendValues.Add("text", text)

	default:
		keywords := getKeywords(incomingText)
		movies := getMovies(keywords)
		sendValues.Add("text", movies)
	}

	response, err := http.PostForm(telegramAPI, sendValues)
	if err != nil {
		log.Printf("error when posting text to the chat: %s", err.Error())
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("error in parsing telegram response %s", err.Error())
		return "", err
	}

	log.Printf("body of the telegram response: %s", string(body))

	return string(body), nil
}

// getMovies constructs an IMDB URL which will be used to scrape movies out of it. it returns list of scraped movies.
func getMovies(keywords []string) string {
	URL := IMDB_URL + keywords[0]
	for i := 1; i < len(keywords); i++ {
		URL += "%2C" + keywords[i]
	}

	c := colly.NewCollector()

	var movies string

	c.OnHTML(`h3[class="lister-item-header"]`, func(element *colly.HTMLElement) {
		movies += strings.TrimSpace(element.DOM.Children().Text()) + "\n"
	})

	c.Visit(URL)

	return movies
}

// getKeywords parses incoming text and returns keywords
func getKeywords(incomingText string) []string {
	incomingText = strings.ReplaceAll(incomingText, " ", "")
	return strings.Split(incomingText, ",")
}
