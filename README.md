# browserhttp - Chrome-backed HTTP Client for Go

[![Go CI](https://github.com/gleicon/browserhttp/actions/workflows/go.yml/badge.svg)](https://github.com/gleicon/browserhttp/actions/workflows/go.yml)

`browserhttp` is a drop-in `http.Client`-compatible Go package that uses a real headless browser (Chrome via [chromedp](https://github.com/chromedp/chromedp)) to fetch and interact with web pages.

Ideal for security testing, scraping, automation, and environments where JavaScript rendering is required, WAFs block standard HTTP clients, or you need to behave exactly like a browser.

## Features

- Drop-in `Do(*http.Request)` compatibility with standard `http.Client`
- Real browser sessions using chromedp
- Support for GET, POST, JSON, and file upload requests
- Advanced browser controls (wait, click, type, evaluate JavaScript)
- Session management with cookie and localStorage support
- Performance monitoring and metrics collection
- Content extraction and SEO analysis
- Security scanning capabilities
- Device emulation and proxy support
- Request/response interception
- Screenshot capture functionality
- Persistent tab/session reuse for multi-request flows
- Comprehensive error handling with retry logic

## Installation

```bash
go get github.com/gleicon/browserhttp
```

## Quick Start

### Basic Usage

```go
package main

import (
    "net/http"
    "time"
    "github.com/gleicon/browserhttp"
)

func main() {
    client := browserhttp.NewClient(15 * time.Second)
    client.Init()
    defer client.Close()
    
    req, _ := http.NewRequest("GET", "https://example.com", nil)
    resp, err := client.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
}
```

### Enhanced Request Methods

```go
// JSON POST request
data := map[string]string{"username": "admin", "password": "secret"}
resp, err := client.PostJSON("https://api.example.com/login", data)

// Custom headers
headers := map[string]string{"Authorization": "Bearer token123"}
resp, err := client.DoWithHeaders(req, headers)

// File upload
file := strings.NewReader("file content")
resp, err := client.PostFile("https://example.com/upload", "file", "document.txt", file)
```

### Browser Automation

```go
// Wait for elements and interact with page
err := client.WaitForElement("#login-form", 5*time.Second)
err = client.Type("#username", "admin")
err = client.Type("#password", "secret")
err = client.Click("#submit-button")

// Execute JavaScript
var result string
err = client.Evaluate("document.title", &result)

// Extract page content
links, err := client.ExtractLinks()
images, err := client.ExtractImages()
forms, err := client.ExtractForms()
```

### Session Management

```go
// Enable persistent tabs for login flows
client.UsePersistentTabs(true)

// Cookie management
cookies, err := client.GetCookies()
err = client.SetCookies(cookies)
err = client.ClearCookies()

// localStorage operations
value, err := client.GetLocalStorage("sessionToken")
err = client.SetLocalStorage("preference", "dark-mode")

// Save/load entire session
err = client.SaveSession("session.json")
err = client.LoadSession("session.json")
```

### Performance and Analysis

```go
// Get performance metrics
metrics, err := client.GetPerformanceMetrics()
fmt.Printf("DOM loaded in: %v\n", metrics.DOMContentLoaded)

// SEO analysis
seo, err := client.AnalyzeSEO()
fmt.Printf("Page title: %s\n", seo.Title)
fmt.Printf("Meta description: %s\n", seo.Description)

// Security scanning
vulns, err := client.DetectVulnerabilities()
csp, err := client.CheckCSP()
ssl, err := client.CheckSSL()
```

### Advanced Configuration

```go
// Device emulation
device := browserhttp.DeviceEmulation{
    UserAgent: "Mozilla/5.0...",
    Width:     375,
    Height:    667,
    Mobile:    true,
    Touch:     true,
}
client.EmulateDevice(device)

// Retry configuration
retryConfig := browserhttp.RetryConfig{
    MaxAttempts: 3,
    Delay:       time.Second,
    Backoff:     true,
}
client.WithRetry(retryConfig)

// Proxy support
client.SetProxy("http://proxy.example.com:8080")

// Request/response interception
client.AddRequestInterceptor(func(req *http.Request) *http.Request {
    req.Header.Set("Custom-Header", "value")
    return req
})
```

## CLI Tool: burl

The package includes `burl`, a curl-like CLI tool with browser capabilities:

```bash
# Build the CLI
make build-burl

# Basic usage
./bin/burl https://example.com

# POST with form data
./bin/burl -X POST -d "user=admin&pass=123" https://httpbin.org/post

# With screenshots and verbose output
./bin/burl -v -s ./screenshots https://example.com

# Persistent session with redirects
./bin/burl -L -p https://site.com/login
```

## Screenshot Capture

Enable automatic screenshot capture for visual documentation:

```go
// Programmatically
os.MkdirAll("./screenshots", 0755)
client.EnableScreenshots("./screenshots")

// Via CLI
burl -s ./screenshots https://example.com
```

Screenshots are saved with timestamped filenames and are useful for penetration testing documentation, debugging, and visual regression testing.

## Environment Configuration

- `CHROME_FLAGS=--no-sandbox` - Disable Chrome sandbox for CI/containers
- `CI=true` - Increase timeout to 60 seconds for slower environments

## Use Cases

- **Security Testing**: Automated vulnerability scanning with real browser behavior
- **Web Scraping**: Handle JavaScript-heavy sites that require browser rendering  
- **Penetration Testing**: Browser-based authentication and session management
- **Quality Assurance**: Automated testing with screenshot documentation
- **API Testing**: Test endpoints that require browser-like requests
- **Content Analysis**: Extract and analyze web page content and structure

## API Reference

### Core Methods
- `NewClient(timeout)` - Create new browser client
- `Init()` - Initialize browser instance
- `Close()` - Clean up browser resources
- `Do(req)` - Execute HTTP request (drop-in replacement)

### Enhanced Requests
- `PostJSON(url, data)` - POST with JSON payload
- `DoWithHeaders(req, headers)` - Request with custom headers
- `PostFile(url, field, filename, file)` - File upload

### Browser Controls
- `WaitForElement(selector, timeout)` - Wait for element to appear
- `WaitForText(text, timeout)` - Wait for text content
- `WaitForNavigation(timeout)` - Wait for page navigation
- `Click(selector)` - Click element
- `Type(selector, text)` - Type into element
- `Select(selector, value)` - Select option
- `Evaluate(script, result)` - Execute JavaScript

### Session Management
- `GetCookies()` - Retrieve all cookies
- `SetCookies(cookies)` - Set cookies
- `ClearCookies()` - Clear all cookies
- `GetLocalStorage(key)` - Get localStorage value
- `SetLocalStorage(key, value)` - Set localStorage value
- `SaveSession(filename)` - Save session to file
- `LoadSession(filename)` - Load session from file

### Analysis and Monitoring
- `GetPerformanceMetrics()` - Get page performance data
- `ExtractLinks()` - Extract all page links
- `ExtractImages()` - Extract all image URLs
- `ExtractForms()` - Extract form information
- `AnalyzeSEO()` - Perform SEO analysis
- `CheckCSP()` - Analyze Content Security Policy
- `CheckSSL()` - Check SSL certificate
- `DetectVulnerabilities()` - Basic vulnerability detection

## Build and Development

```bash
# Install dependencies
make deps

# Build library
make build

# Run tests
make test

# Build examples
make build-examples

# Build CLI tool
make build-burl

# Clean build artifacts
make clean
```

## Contributing

Pull requests welcome! Please ensure backward compatibility with existing `http.Client` interface and include tests for new functionality.

## Author

[gleicon](https://github.com/gleicon)