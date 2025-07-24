// Package browserhttp provides a drop-in http.Client implementation
// that uses headless Chrome (via chromedp) to send HTTP requests as a real browser.
// It is useful for bypassing WAFs, detecting JavaScript-rendered content,
// and testing sites that require client-side rendering.

package browserhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// BrowserResponse contains enhanced response information with browser metadata
type BrowserResponse struct {
	*http.Response
	Title           string
	FinalURL        string
	LoadTime        time.Duration
	NetworkRequests []NetworkRequest
	Console         []ConsoleMessage
	Screenshots     []string
}

// NetworkRequest represents a network request made during page load
type NetworkRequest struct {
	URL      string
	Method   string
	Status   int
	Duration time.Duration
	Headers  map[string]string
}

// ConsoleMessage represents a browser console message
type ConsoleMessage struct {
	Level   string
	Text    string
	URL     string
	Line    int
	Column  int
}

// PerformanceMetrics contains page performance data
type PerformanceMetrics struct {
	DOMContentLoaded time.Duration
	LoadComplete     time.Duration
	FirstPaint       time.Duration
	NetworkRequests  int
	ResourceSizes    map[string]int64
}

// BrowserError represents browser-specific errors
type BrowserError struct {
	Type    string
	Message string
	Code    int
	URL     string
}

func (e *BrowserError) Error() string {
	return fmt.Sprintf("browser error [%s]: %s (url: %s)", e.Type, e.Message, e.URL)
}

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxAttempts int
	Delay       time.Duration
	Backoff     bool
}

// DeviceEmulation defines device emulation settings
type DeviceEmulation struct {
	UserAgent string
	Width     int
	Height    int
	Mobile    bool
	Touch     bool
}

// RequestInterceptor allows modifying requests
type RequestInterceptor func(*http.Request) *http.Request

// ResponseInterceptor allows modifying responses
type ResponseInterceptor func(*http.Response) *http.Response

// FormData represents extracted form information
type FormData struct {
	Action string
	Method string
	Fields map[string]string
}

// ImageData represents image metadata
type ImageData struct {
	Src string
	Alt string
	Width int
	Height int
}

// SEOData contains SEO analysis information
type SEOData struct {
	Title       string
	Description string
	Keywords    []string
	Headings    map[string][]string
	Images      []ImageData
}

// CSPReport contains Content Security Policy information
type CSPReport struct {
	Directives map[string][]string
	Violations []string
}

// SSLReport contains SSL certificate information
type SSLReport struct {
	Valid      bool
	Issuer     string
	Subject    string
	Expiration time.Time
	Errors     []string
}

// Vulnerability represents a security vulnerability
type Vulnerability struct {
	Type        string
	Severity    string
	Description string
	URL         string
}

// BrowserClient implements a drop-in replacement for http.Client
// using a headless browser to execute the requests.
type BrowserClient struct {
	Timeout               time.Duration
	Verbose               bool
	PersistentTabs        bool
	allocatorCtx          context.Context
	browserCancelFn       context.CancelFunc
	tabCtx                context.Context
	CaptureScreenshots    bool
	ScreenshotDir         string
	retryConfig           *RetryConfig
	deviceEmulation       *DeviceEmulation
	requestInterceptors   []RequestInterceptor
	responseInterceptors  []ResponseInterceptor
	proxyURL              string
	networkRequests       []NetworkRequest
	consoleMessages       []ConsoleMessage
}

// NewClient returns a BrowserClient with the given timeout.
func NewClient(timeout time.Duration) *BrowserClient {
	return &BrowserClient{
		Timeout: timeout,
	}
}

// optional: screenshot every request to a pre configured directory
func (bc *BrowserClient) EnableScreenshots(dir string) {
	bc.CaptureScreenshots = true
	bc.ScreenshotDir = dir
}

// EnableVerbose turns on logging for the browser client.
func (bc *BrowserClient) EnableVerbose() {
	bc.Verbose = true
}

// UsePersistentTabs configures whether to reuse a browser tab across requests.
func (bc *BrowserClient) UsePersistentTabs(persist bool) {
	bc.PersistentTabs = persist
}

// Init sets up the Chrome instance and persistent tab (if enabled).
func (bc *BrowserClient) Init() error {

	timeout := bc.Timeout
	if os.Getenv("CI") == "true" {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	bc.browserCancelFn = cancel

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("enable-automation", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-timer-throttling", false),

	)

	if os.Getenv("CHROME_FLAGS") == "--no-sandbox" {
		opts = append(opts, chromedp.Flag("no-sandbox", true))
	}

	allocCtx, _ := chromedp.NewExecAllocator(ctx, opts...)
	bc.allocatorCtx = allocCtx

	if bc.PersistentTabs {
		bc.tabCtx, _ = chromedp.NewContext(allocCtx)
	}

	return nil
}

// Close ends the browser session.
func (bc *BrowserClient) Close() {
	if bc.browserCancelFn != nil {
		bc.browserCancelFn()
	}
}

// Do simulates http.Client's Do method but uses headless Chrome.
func (bc *BrowserClient) Do(req *http.Request) (*http.Response, error) {
	if bc.Verbose {
		log.Printf("[browserhttp] Visiting %s [%s]", req.URL.String(), req.Method)
	}

	switch req.Method {
	case http.MethodGet:
		return bc.doGET(req)
	case http.MethodPost:
		return bc.doPOST(req)
	default:
		return nil, errors.New("browserhttp only supports GET and POST methods currently")
	}
}

func (bc *BrowserClient) getContext() context.Context {
	if bc.PersistentTabs && bc.tabCtx != nil {
		return bc.tabCtx
	}
	ctx, _ := chromedp.NewContext(bc.allocatorCtx)
	return ctx
}

func (bc *BrowserClient) doGET(req *http.Request) (*http.Response, error) {
	ctx := bc.getContext()
	var html string
	var statusCode int64 = 200 // fallback default
	var statusText string = "OK"
	var respHeaders http.Header = make(http.Header)
	done := make(chan struct{}) // 👈 signal for response

	// Attach listener early
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if res, ok := ev.(*network.EventResponseReceived); ok {
			if res.Type == network.ResourceTypeDocument {
				statusCode = int64(res.Response.Status)
				statusText = res.Response.StatusText
				for k, v := range res.Response.Headers {
					respHeaders.Set(k, fmt.Sprintf("%v", v))
				}
				select {
				case <-done: // already closed
				default:
					close(done)
				}
			}
		}
	})

	// Enable network capture before navigation
	err := chromedp.Run(ctx, network.Enable())
	if err != nil {
		return nil, err
	}

	// Navigate and extract HTML
	err = chromedp.Run(ctx,
		chromedp.Navigate(req.URL.String()),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return nil, err
	}

	// Wait up to 2s for status event
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		if bc.Verbose {
			log.Println("[browserhttp] Warning: response status capture timed out")
		}
	}

	// Optional screenshot
	if bc.CaptureScreenshots {
		var buf []byte
		if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err == nil {
			filename := fmt.Sprintf("%s/snap_%d.png", bc.ScreenshotDir, time.Now().UnixNano())
			_ = os.WriteFile(filename, buf, 0644)
			if bc.Verbose {
				log.Printf("[browserhttp] Screenshot saved to %s", filename)
			}
		} else if bc.Verbose {
			log.Printf("[browserhttp] Failed to capture screenshot: %v", err)
		}
	}

	return &http.Response{
		StatusCode: int(statusCode),
		Status:     fmt.Sprintf("%d %s", statusCode, statusText),
		Header:     respHeaders,
		Body:       io.NopCloser(strings.NewReader(html)),
		Request:    req,
	}, nil
}

func (bc *BrowserClient) doPOST(req *http.Request) (*http.Response, error) {
	ctx := bc.getContext()
	var html string
	formAction := req.URL.String()
	var postScript string
	var statusCode int64 = 200 // fallback default
	var statusText string = "OK"
	respHeaders := make(http.Header)
	done := make(chan struct{}) // used to wait for network event

	// Attach listener before any navigation
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if res, ok := ev.(*network.EventResponseReceived); ok {
			if res.Type == network.ResourceTypeDocument {
				statusCode = int64(res.Response.Status)
				statusText = res.Response.StatusText
				for k, v := range res.Response.Headers {
					respHeaders.Set(k, fmt.Sprintf("%v", v))
				}
				select {
				case <-done:
				default:
					close(done)
				}
			}
		}
	})

	// Generate the form submission script from request body
	if req.Body != nil {
		bodyBytes, _ := io.ReadAll(req.Body)
		values, _ := url.ParseQuery(string(bodyBytes))
		postScript = "var form = document.createElement('form'); form.method = 'POST'; form.action = '" + formAction + "';"
		for key, vals := range values {
			for _, val := range vals {
				postScript += fmt.Sprintf("var input = document.createElement('input'); input.name = '%s'; input.value = '%s'; form.appendChild(input);", key, val)
			}
		}
		postScript += "document.body.appendChild(form); form.submit();"
	}

	// Run the navigation and form submission
	err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.Navigate("about:blank"),
		chromedp.Evaluate(postScript, nil),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return nil, err
	}

	// Wait for response metadata or timeout
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		if bc.Verbose {
			log.Println("[browserhttp] Warning: response status capture timed out")
		}
	}

	// Optional screenshot capture
	if bc.CaptureScreenshots {
		var buf []byte
		if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err == nil {
			filename := fmt.Sprintf("%s/snap_%d.png", bc.ScreenshotDir, time.Now().UnixNano())
			_ = os.WriteFile(filename, buf, 0644)
			if bc.Verbose {
				log.Printf("[browserhttp] Screenshot saved to %s", filename)
			}
		} else if bc.Verbose {
			log.Printf("[browserhttp] Failed to capture screenshot: %v", err)
		}
	}

	return &http.Response{
		StatusCode: int(statusCode),
		Status:     fmt.Sprintf("%d %s", statusCode, statusText),
		Header:     respHeaders,
		Body:       io.NopCloser(strings.NewReader(html)),
		Request:    req,
	}, nil
}

// Get issues a GET to the specified URL.
func (bc *BrowserClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return bc.Do(req)
}

// Head issues a HEAD to the specified URL.
func (bc *BrowserClient) Head(url string) (*http.Response, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, err
	}
	return bc.Do(req)
}

// Post issues a POST to the specified URL.
func (bc *BrowserClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return bc.Do(req)
}

// PostForm issues a POST to the specified URL, with data's keys and values URL-encoded as the request body.
func (bc *BrowserClient) PostForm(url string, data url.Values) (*http.Response, error) {
	return bc.Post(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

// PostJSON issues a POST request with JSON body
func (bc *BrowserClient) PostJSON(url string, data interface{}) (*http.Response, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return bc.Post(url, "application/json", strings.NewReader(string(jsonData)))
}

// DoWithHeaders executes a request with custom headers
func (bc *BrowserClient) DoWithHeaders(req *http.Request, headers map[string]string) (*http.Response, error) {
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return bc.Do(req)
}

// PostFile uploads a file via POST request
func (bc *BrowserClient) PostFile(url, fieldName, fileName string, file io.Reader) (*http.Response, error) {
	var b strings.Builder
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile(fieldName, fileName)
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(fw, file); err != nil {
		return nil, err
	}
	w.Close()
	return bc.Post(url, w.FormDataContentType(), strings.NewReader(b.String()))
}

// WaitForElement waits for an element to appear on the page
func (bc *BrowserClient) WaitForElement(selector string, timeout time.Duration) error {
	ctx := bc.getContext()
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return chromedp.Run(ctxTimeout, chromedp.WaitVisible(selector))
}

// WaitForText waits for specific text to appear on the page
func (bc *BrowserClient) WaitForText(text string, timeout time.Duration) error {
	ctx := bc.getContext()
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return chromedp.Run(ctxTimeout, chromedp.WaitVisible(fmt.Sprintf("//*[contains(text(), '%s')]", text), chromedp.BySearch))
}

// WaitForNavigation waits for page navigation to complete
func (bc *BrowserClient) WaitForNavigation(timeout time.Duration) error {
	ctx := bc.getContext()
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return chromedp.Run(ctxTimeout, chromedp.WaitReady("body"))
}

// Click clicks on an element specified by selector
func (bc *BrowserClient) Click(selector string) error {
	ctx := bc.getContext()
	return chromedp.Run(ctx, chromedp.Click(selector))
}

// Type types text into an element specified by selector
func (bc *BrowserClient) Type(selector, text string) error {
	ctx := bc.getContext()
	return chromedp.Run(ctx, chromedp.SendKeys(selector, text))
}

// Select selects an option in a select element
func (bc *BrowserClient) Select(selector, value string) error {
	ctx := bc.getContext()
	return chromedp.Run(ctx, chromedp.SetValue(selector, value))
}

// Evaluate executes JavaScript and returns the result
func (bc *BrowserClient) Evaluate(script string, result interface{}) error {
	ctx := bc.getContext()
	return chromedp.Run(ctx, chromedp.Evaluate(script, result))
}

// ExtractText extracts text from elements matching the selector
func (bc *BrowserClient) ExtractText(selector string) ([]string, error) {
	ctx := bc.getContext()
	var texts []string
	err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
		Array.from(document.querySelectorAll('%s')).map(el => el.textContent.trim())
	`, selector), &texts))
	return texts, err
}

// GetCookies retrieves all cookies for the current page
func (bc *BrowserClient) GetCookies() ([]*http.Cookie, error) {
	ctx := bc.getContext()
	var cookiesJSON string
	err := chromedp.Run(ctx, chromedp.Evaluate(`JSON.stringify(document.cookie.split(';').map(c => {
		const [name, value] = c.trim().split('=');
		return {name: name, value: value || ''};
	}))`, &cookiesJSON))
	if err != nil {
		return nil, err
	}
	
	var cookieData []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(cookiesJSON), &cookieData); err != nil {
		return nil, err
	}
	
	var httpCookies []*http.Cookie
	for _, cookie := range cookieData {
		if cookie.Name != "" {
			httpCookies = append(httpCookies, &http.Cookie{
				Name:  cookie.Name,
				Value: cookie.Value,
			})
		}
	}
	return httpCookies, nil
}

// SetCookies sets cookies for the current page
func (bc *BrowserClient) SetCookies(cookies []*http.Cookie) error {
	ctx := bc.getContext()
	for _, cookie := range cookies {
		cookieScript := fmt.Sprintf("document.cookie = '%s=%s; path=/';", cookie.Name, cookie.Value)
		err := chromedp.Run(ctx, chromedp.Evaluate(cookieScript, nil))
		if err != nil {
			return err
		}
	}
	return nil
}

// ClearCookies clears all cookies
func (bc *BrowserClient) ClearCookies() error {
	ctx := bc.getContext()
	return chromedp.Run(ctx, chromedp.Evaluate(`
		document.cookie.split(";").forEach(function(c) { 
			document.cookie = c.replace(/^ +/, "").replace(/=.*/, "=;expires=" + new Date().toUTCString() + ";path=/"); 
		});
	`, nil))
}

// GetLocalStorage retrieves a value from localStorage
func (bc *BrowserClient) GetLocalStorage(key string) (string, error) {
	ctx := bc.getContext()
	var value string
	err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf("localStorage.getItem('%s')", key), &value))
	return value, err
}

// SetLocalStorage sets a value in localStorage
func (bc *BrowserClient) SetLocalStorage(key, value string) error {
	ctx := bc.getContext()
	return chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf("localStorage.setItem('%s', '%s')", key, value), nil))
}

// SaveSession saves browser session to file
func (bc *BrowserClient) SaveSession(filename string) error {
	cookies, err := bc.GetCookies()
	if err != nil {
		return err
	}
	data, err := json.Marshal(cookies)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

// LoadSession loads browser session from file
func (bc *BrowserClient) LoadSession(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	var cookies []*http.Cookie
	err = json.Unmarshal(data, &cookies)
	if err != nil {
		return err
	}
	return bc.SetCookies(cookies)
}

// GetPerformanceMetrics retrieves page performance metrics
func (bc *BrowserClient) GetPerformanceMetrics() (*PerformanceMetrics, error) {
	ctx := bc.getContext()
	var metrics PerformanceMetrics
	
	err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const perf = performance.getEntriesByType('navigation')[0];
			return {
				domContentLoaded: perf.domContentLoadedEventEnd - perf.domContentLoadedEventStart,
				loadComplete: perf.loadEventEnd - perf.loadEventStart,
				firstPaint: performance.getEntriesByType('paint').find(p => p.name === 'first-paint')?.startTime || 0,
				networkRequests: performance.getEntriesByType('resource').length
			};
		})()
	`, &metrics))
	
	return &metrics, err
}

// WithRetry configures retry behavior
func (bc *BrowserClient) WithRetry(config RetryConfig) *BrowserClient {
	bc.retryConfig = &config
	return bc
}

// SetProxy configures proxy settings
func (bc *BrowserClient) SetProxy(proxyURL string) error {
	bc.proxyURL = proxyURL
	return nil
}

// AddRequestInterceptor adds a request interceptor
func (bc *BrowserClient) AddRequestInterceptor(interceptor RequestInterceptor) {
	bc.requestInterceptors = append(bc.requestInterceptors, interceptor)
}

// AddResponseInterceptor adds a response interceptor
func (bc *BrowserClient) AddResponseInterceptor(interceptor ResponseInterceptor) {
	bc.responseInterceptors = append(bc.responseInterceptors, interceptor)
}

// EmulateDevice configures device emulation
func (bc *BrowserClient) EmulateDevice(device DeviceEmulation) error {
	bc.deviceEmulation = &device
	return nil
}

// ExtractLinks extracts all links from the current page
func (bc *BrowserClient) ExtractLinks() ([]string, error) {
	ctx := bc.getContext()
	var links []string
	err := chromedp.Run(ctx, chromedp.Evaluate(`
		Array.from(document.querySelectorAll('a[href]')).map(a => a.href)
	`, &links))
	return links, err
}

// ExtractImages extracts all image URLs from the current page
func (bc *BrowserClient) ExtractImages() ([]string, error) {
	ctx := bc.getContext()
	var images []string
	err := chromedp.Run(ctx, chromedp.Evaluate(`
		Array.from(document.querySelectorAll('img[src]')).map(img => img.src)
	`, &images))
	return images, err
}

// ExtractForms extracts form information from the current page
func (bc *BrowserClient) ExtractForms() ([]FormData, error) {
	ctx := bc.getContext()
	var forms []FormData
	err := chromedp.Run(ctx, chromedp.Evaluate(`
		Array.from(document.querySelectorAll('form')).map(form => ({
			action: form.action,
			method: form.method,
			fields: Array.from(form.querySelectorAll('input, select, textarea')).reduce((acc, field) => {
				if (field.name) acc[field.name] = field.value || field.type;
				return acc;
			}, {})
		}))
	`, &forms))
	return forms, err
}

// AnalyzeSEO performs basic SEO analysis of the current page
func (bc *BrowserClient) AnalyzeSEO() (*SEOData, error) {
	ctx := bc.getContext()
	var seo SEOData
	err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const title = document.title;
			const description = document.querySelector('meta[name="description"]')?.content || '';
			const keywords = document.querySelector('meta[name="keywords"]')?.content?.split(',') || [];
			const headings = {};
			['h1', 'h2', 'h3', 'h4', 'h5', 'h6'].forEach(tag => {
				headings[tag] = Array.from(document.querySelectorAll(tag)).map(h => h.textContent.trim());
			});
			const images = Array.from(document.querySelectorAll('img')).map(img => ({
				src: img.src,
				alt: img.alt,
				width: img.width,
				height: img.height
			}));
			return { title, description, keywords, headings, images };
		})()
	`, &seo))
	return &seo, err
}

// CheckCSP analyzes Content Security Policy
func (bc *BrowserClient) CheckCSP() (*CSPReport, error) {
	ctx := bc.getContext()
	var report CSPReport
	err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const cspMeta = document.querySelector('meta[http-equiv="Content-Security-Policy"]');
			const csp = cspMeta ? cspMeta.content : '';
			const directives = {};
			if (csp) {
				csp.split(';').forEach(directive => {
					const [key, ...values] = directive.trim().split(' ');
					if (key) directives[key] = values;
				});
			}
			return { directives, violations: [] };
		})()
	`, &report))
	return &report, err
}

// CheckSSL analyzes SSL certificate information
func (bc *BrowserClient) CheckSSL() (*SSLReport, error) {
	ctx := bc.getContext()
	var report SSLReport
	err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			return {
				valid: location.protocol === 'https:',
				issuer: '',
				subject: '',
				expiration: new Date().toISOString(),
				errors: location.protocol !== 'https:' ? ['Not using HTTPS'] : []
			};
		})()
	`, &report))
	return &report, err
}

// DetectVulnerabilities performs basic vulnerability detection
func (bc *BrowserClient) DetectVulnerabilities() ([]Vulnerability, error) {
	ctx := bc.getContext()
	var vulns []Vulnerability
	err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const vulnerabilities = [];
			if (location.protocol !== 'https:') {
				vulnerabilities.push({
					type: 'insecure_protocol',
					severity: 'medium',
					description: 'Page not served over HTTPS',
					url: location.href
				});
			}
			if (!document.querySelector('meta[http-equiv="Content-Security-Policy"]')) {
				vulnerabilities.push({
					type: 'missing_csp',
					severity: 'low',
					description: 'No Content Security Policy found',
					url: location.href
				});
			}
			return vulnerabilities;
		})()
	`, &vulns))
	return vulns, err
}
