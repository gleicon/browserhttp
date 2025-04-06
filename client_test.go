package browserhttp_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gleicon/browserhttp"
)

func TestGETRequest(t *testing.T) {
	client := browserhttp.NewClient(15 * time.Second)
	err := client.Init()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer client.Close()

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestPOSTRequest(t *testing.T) {
	client := browserhttp.NewClient(15 * time.Second)
	client.UsePersistentTabs(true)
	err := client.Init()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer client.Close()

	data := "name=test&value=123"
	req, _ := http.NewRequest("POST", "https://httpbin.org/post", strings.NewReader(data))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

