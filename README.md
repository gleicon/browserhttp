![browserhttp](https://raw.githubusercontent.com/gleicon/browserhttp/main/logo.png)

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
req, _ := http.NewRequest("GET", "https://example.com", nil)
resp, _ := client.Do(req)
```

### POST Form
```go
data := "username=admin&password=secret"
req, _ := http.NewRequest("POST", "https://target.com/login", strings.NewReader(data))
req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
resp, _ := client.Do(req)
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

# Follow redirects
burl -L https://site-with-redirects.com
```

---

## 🔧 Internals
- `doGET()` uses `chromedp.Navigate()` and `chromedp.OuterHTML()`
- `doPOST()` simulates JS form creation and submission

---

## 🚧 TODO
- Support JSON and custom body types
- Capture real response headers from browser context
- Add session/cookie persistence

---

## 🛡️ Use Cases
- Automated OWASP scans
- Web scraping (JS-only pages)
- Red team tools

---

## 🧠 Author
[gleicon](https://github.com/gleicon)

Pull requests welcome!


