package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type JokeInfo struct {
	ID     string `json:"id"`
	Joke   string `json:"joke"`
	Status int    `json:"status"`
}

const URL string = "https://www.icanhazdadjoke.com"

func dadJoke() string {
	client := http.Client{}

	var JokeData JokeInfo

	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(body, &JokeData)
	if err != nil {
		log.Fatal(err)
	}

	return fmt.Sprintf("Joke: %s", JokeData.Joke)
}
