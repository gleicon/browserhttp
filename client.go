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
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// BrowserClient implements a drop-in replacement for http.Client
// using a headless browser to execute the requests.
type BrowserClient struct {
	Timeout         time.Duration
	Verbose         bool
	PersistentTabs  bool
	allocatorCtx    context.Context
	browserCancelFn context.CancelFunc
	tabCtx context.Context
}

// NewClient returns a BrowserClient with the given timeout.
func NewClient(timeout time.Duration) *BrowserClient {
	return &BrowserClient{
		Timeout: timeout,
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), bc.Timeout)
	bc.browserCancelFn = cancel

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
	)
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

	err := chromedp.Run(ctx,
		chromedp.Navigate(req.URL.String()),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(html)),
		Request:    req,
	}, nil
}

func (bc *BrowserClient) doPOST(req *http.Request) (*http.Response, error) {
	ctx := bc.getContext()
	var html string
	formAction := req.URL.String()
	var postScript string

	if req.Body != nil {
		bodyBytes, _ := ioutil.ReadAll(req.Body)
		values, _ := url.ParseQuery(string(bodyBytes))
		postScript = "var form = document.createElement('form'); form.method = 'POST'; form.action = '" + formAction + "';"
		for key, vals := range values {
			for _, val := range vals {
				postScript += fmt.Sprintf("var input = document.createElement('input'); input.name = '%s'; input.value = '%s'; form.appendChild(input);", key, val)
			}
		}
		postScript += "document.body.appendChild(form); form.submit();"
	}

	err := chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Evaluate(postScript, nil),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(html)),
		Request:    req,
	}, nil
}

