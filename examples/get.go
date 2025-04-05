package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gleicon/browserhttp"
)

func main() {
	client := browserhttp.NewClient(15 * time.Second)

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
