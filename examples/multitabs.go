package main

import (
	"fmt"
	"io"
	"net/http"
	"time"
  "strings"
	"github.com/gleicon/browserhttp"
)

func main() {
	client := browserhttp.NewClient(20 * time.Second)
	client.UsePersistentTabs(true)
	client.EnableVerbose()
	client.Init()
	defer client.Close()

	// Simulate multiple steps in the same tab session
	// 1. Visit login page (just load it)
	req1, _ := http.NewRequest("GET", "https://example.com/login", nil)
	resp1, err := client.Do(req1)
	if err != nil {
		panic(err)
	}
	resp1.Body.Close()

	// 2. Perform login using POST form
	formData := "username=admin&password=secret"
	req2, _ := http.NewRequest("POST", "https://example.com/login", strings.NewReader(formData))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp2, err := client.Do(req2)
	if err != nil {
		panic(err)
	}
	resp2.Body.Close()

	// 3. Now access a page that requires a session (dashboard)
	req3, _ := http.NewRequest("GET", "https://example.com/dashboard", nil)
	resp3, err := client.Do(req3)
	if err != nil {
		panic(err)
	}
	body, _ := io.ReadAll(resp3.Body)
	resp3.Body.Close()

	fmt.Println("Dashboard content:")
	fmt.Println(string(body))
}
