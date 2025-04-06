package main

import (
	"io"
	"net/http"
	"time"

	"github.com/gleicon/browserhttp"
)

func main() {
	client := browserhttp.NewClient(10 * time.Second)
	client.UsePersistentTabs(false)
	client.EnableVerbose()
	client.Init()
	defer client.Close()

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	println(string(body))
}
