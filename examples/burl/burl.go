package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gleicon/browserhttp"
)

func main() {
	method := flag.String("X", "GET", "HTTP method to use (GET or POST)")
	data := flag.String("d", "", "POST data (application/x-www-form-urlencoded)")
	verbose := flag.Bool("v", false, "Enable verbose output")
	showHeaders := flag.Bool("i", false, "Show response headers")
	headerOut := flag.String("H", "", "Save response headers to file")
	bodyOut := flag.String("o", "", "Save response body to file")
	followRedirect := flag.Bool("L", false, "Follow redirects")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Println("Usage: burl [options] <URL>")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	targetURL := flag.Arg(0)
	client := browserhttp.NewClient(20 * time.Second)
	if *verbose {
		client.EnableVerbose()
	}

	var body io.Reader
	if *data != "" {
		body = strings.NewReader(*data)
	}

	req, err := http.NewRequest(strings.ToUpper(*method), targetURL, body)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	if *data != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	handled := false
	for !handled {
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if *showHeaders {
			fmt.Printf("HTTP/1.1 %s\n", resp.Status)
			for key, values := range resp.Header {
				for _, val := range values {
					fmt.Printf("%s: %s\n", key, val)
				}
			}
			fmt.Println()
		}

		// Save headers if requested
		if *headerOut != "" {
			hf, err := os.Create(*headerOut)
			if err != nil {
				log.Fatalf("Failed to create header file: %v", err)
			}
			defer hf.Close()
			fmt.Fprintf(hf, "HTTP/1.1 %s\n", resp.Status)
			for key, values := range resp.Header {
				for _, val := range values {
					fmt.Fprintf(hf, "%s: %s\n", key, val)
				}
			}
		}

		output, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Failed to read response: %v", err)
		}

		if *bodyOut != "" {
			bf, err := os.Create(*bodyOut)
			if err != nil {
				log.Fatalf("Failed to create output file: %v", err)
			}
			defer bf.Close()
			bf.Write(output)
		} else {
			fmt.Println(string(output))
		}

		// Handle redirect
		if *followRedirect {
			if resp.StatusCode >= 300 && resp.StatusCode < 400 {
				loc := resp.Header.Get("Location")
				if loc == "" {
					break
				}
				if *verbose {
					fmt.Printf("[burl] Following redirect to: %s\n", loc)
				}
				req, _ = http.NewRequest("GET", loc, nil)
				continue
			}
		}
		handled = true
	}
}

