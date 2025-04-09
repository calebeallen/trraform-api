package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func LogErrorDiscord(endpoint string, errLog error, jsonPayload any) {

	log.Println(errLog)

	msg := fmt.Sprintf("=================================\n\nError in **%s**\n\nLog:\n```\n%v\n```", endpoint, errLog)

	if jsonPayload != nil {

		jsonPayloadFormatted, err := json.MarshalIndent(jsonPayload, "", "  ")
		if err != nil {
			log.Printf("in LogErrorDiscord:\n%v", err)
			return
		}

		msg += fmt.Sprintf("\nRequest body:\n```json\n%s\n```", string(jsonPayloadFormatted))

	}

	payload := map[string]string{"content": msg}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("error marshaling payload:\n%v\n", err)
		return
	}
	buf := bytes.NewBuffer(payloadBytes)

	req, err := http.NewRequest("POST", fmt.Sprintf("https://discord.com/api/channels/%s/messages", os.Getenv("DISCORD_ERROR_LOG_CHANNEL")), buf)
	if err != nil {
		log.Printf("in LogErrorDiscord:\n%v", err)
		return
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bot %s", os.Getenv("DISCORD_BOT_TOKEN")))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("in LogErrorDiscord:\n%v", err)
		return
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		log.Printf("in LogErrorDiscord:\nreceived non-success status code on send: %d", res.StatusCode)
	}

}
