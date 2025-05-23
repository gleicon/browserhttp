// Package browserhttp provides a drop-in http.Client implementation
// that uses headless Chrome (via chromedp) to send HTTP requests as a real browser.
// It is useful for bypassing WAFs, detecting JavaScript-rendered content,
// and testing sites that require client-side rendering.

package browserhttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// BrowserClient implements a drop-in replacement for http.Client
// using a headless browser to execute the requests.
type BrowserClient struct {
	Timeout            time.Duration
	Verbose            bool
	PersistentTabs     bool
	allocatorCtx       context.Context
	browserCancelFn    context.CancelFunc
	tabCtx             context.Context
	CaptureScreenshots bool
	ScreenshotDir      string
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
