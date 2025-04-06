![browserhttp](https://raw.githubusercontent.com/gleicon/browserhttp/main/logo.jpeg)

# browserhttp

[![Go CI](https://github.com/gleicon/browserhttp/actions/workflows/go.yml/badge.svg)](https://github.com/gleicon/browserhttp/actions/workflows/go.yml)

`browserhttp` is a drop-in `http.Client`-compatible Go package that uses a real headless browser (Chrome via [chromedp](https://github.com/chromedp/chromedp)) under the hood to fetch and interact with web pages.

It is ideal for security testing, scraping, automation, and environments where:
- JavaScript rendering is required
- WAFs or anti-bot protection block `http.Client`
- You want to behave exactly like a browser (including cookies, rendering, DOM evaluation)

---

## ✨ Features
- Drop-in `Do(*http.Request)` compatible
- Executes real browser sessions using `chromedp`
- Supports GET and POST requests
- JavaScript form submission simulation
- Logs requests with `EnableVerbose()`
- ✅ Supports persistent tab/session reuse (multi-request flows)
- Designed for use in scanners, red team tools, or web automation

---

## 📦 Installation

```bash
go get github.com/gleicon/browserhttp
```

---

## 🧪 Usage

### Basic GET
```go
client := browserhttp.NewClient(10 * time.Second)
client.Init()
req, _ := http.NewRequest("GET", "https://example.com", nil)
resp, _ := client.Do(req)
```

### Persistent Tab Session (Login Flow)
```go
client := browserhttp.NewClient(15 * time.Second)
client.UsePersistentTabs(true)
client.Init()
defer client.Close()

// Login
req1, _ := http.NewRequest("POST", "https://example.com/login", strings.NewReader("user=admin&pass=secret"))
client.Do(req1)

// Reuse session to access authenticated page
req2, _ := http.NewRequest("GET", "https://example.com/dashboard", nil)
client.Do(req2)
```

### Verbose Mode
```go
client := browserhttp.NewClient(10 * time.Second)
client.EnableVerbose()
```

---

## 📁 Examples

```bash
make build
./bin/burl -v -X POST -d "test=1" https://httpbin.org/post
```

---

## CLI: `burl`

```bash
# Basic GET
burl https://example.com

# POST with form data
burl -X POST -d "user=admin&pass=123" https://httpbin.org/post

# Save output and headers
burl -i -o page.html -H headers.txt https://target.com

# Follow redirects + persistent tab
burl -L -p https://site.com
```

---

## 🔧 Internals
- `doGET()` uses `chromedp.Navigate()` and `chromedp.OuterHTML()`
- `doPOST()` simulates JS form creation and submission
- `UsePersistentTabs(true)` enables tab reuse and session sharing
- $ export CHROME_FLAGS=--no-sandbox disable chrome sandbox, initially adapted to ease CI but can help embedded systems.
---

## 🚧 TODO
- Support JSON and custom body types
- Capture real response headers from browser context
- Add session/cookie persistence

---

## 🛡️ Use Cases
- Automated OWASP scans
- Web scraping (JS-only pages)
- Red team tools with simulated logins
- Browser-based pen testing CLI

---

## 🧠 Author
[gleicon](https://github.com/gleicon)

Pull requests welcome!

