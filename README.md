# browserhttp

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
Run each file from the `examples/` directory:
```bash
go run examples/get.go
```

### `examples/get.go`
Basic page request

### `examples/post.go`
POST request using form simulation

### `examples/verbose.go`
Same as `get.go` but with logging enabled

---

## 🔧 Internals
- `doGET()` uses `chromedp.Navigate()` and `chromedp.OuterHTML()`
- `doPOST()` simulates JS form creation and submission via `EvaluateAsDevTools`
- `io.NopCloser(strings.NewReader(...))` wraps the output back as `http.Response.Body`

---

## 🚧 TODO
- Support JSON and custom body types
- Intercept and modify requests via `Fetch` domain
- Capture HTTP response codes and headers from network events
- Reuse browser context across requests (session-aware client)

---

## 🛡️ Use Cases
- Automated OWASP scans
- Web scraping (JS-only pages)
- Red team infrastructure emulating browser traffic
- API testers that require full page context

---

## 🧠 Author
[gleicon](https://github.com/gleicon)

PRs welcome!



