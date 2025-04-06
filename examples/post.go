package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gleicon/browserhttp"
)

func main() {
	client := browserhttp.NewClient(15 * time.Second)
	client.UsePersistentTabs(true)
	client.Init()
	defer client.Close()

	data := "username=admin&password=secret"
	req, _ := http.NewRequest("POST", "https://httpbin.org/post", strings.NewReader(data))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
