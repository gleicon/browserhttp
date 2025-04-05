// Package browserhttp provides a drop-in http.Client implementation
// that uses headless Chrome (via chromedp) to send HTTP requests as a real browser.
// It is useful for bypassing WAFs, detecting JavaScript-rendered content,
// and testing sites that require client-side rendering.

package browserhttp

import (
	"bytes"
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
	Timeout time.Duration
	Verbose bool
}

// NewClient returns a BrowserClient with the given timeout and optional verbosity.
func NewClient(timeout time.Duration) *BrowserClient {
	return &BrowserClient{Timeout: timeout, Verbose: false}
}

// EnableVerbose turns on logging for the browser client.
func (bc *BrowserClient) EnableVerbose() {
	bc.Verbose = true
}

// Do simulates http.Client's Do method but uses headless Chrome to fetch the page.
func (bc *BrowserClient) Do(req *http.Request) (*http.Response, error) {
	if bc.Verbose {
		log.Printf("[browserhttp] Visiting %s [%s]", req.URL.String(), req.Method)
	}

	if req.Method == http.MethodGet {
		return bc.doGET(req)
	} else if req.Method == http.MethodPost {
		return bc.doPOST(req)
	}
	return nil, errors.New("browserhttp only supports GET and POST methods currently")
}

func (bc *BrowserClient) doGET(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), bc.Timeout)
	defer cancel()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.UserAgent(req.UserAgent()),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	var htmlContent string
	tasks := []chromedp.Action{
		chromedp.Navigate(req.URL.String()),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
	}

	if err := chromedp.Run(taskCtx, tasks...); err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(htmlContent)),
		Request:    req,
	}, nil
}

func (bc *BrowserClient) doPOST(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), bc.Timeout)
	defer cancel()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.UserAgent(req.UserAgent()),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	// Convert the POST body into form fields (x-www-form-urlencoded assumed)
	var formAction = req.URL.String()
	var postScript string

	if req.Body != nil {
		bodyBytes, _ := ioutil.ReadAll(req.Body)
		req.Body.Close()
		req.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		values, _ := url.ParseQuery(string(bodyBytes))

		postScript = "var form = document.createElement('form'); form.method = 'POST'; form.action = '" + formAction + "';"
		for key, vals := range values {
			for _, v := range vals {
				postScript += fmt.Sprintf("var input = document.createElement('input'); input.name = '%s'; input.value = '%s'; form.appendChild(input);", key, v)
			}
		}
		postScript += "document.body.appendChild(form); form.submit();"
	}

	var htmlContent string
	tasks := []chromedp.Action{
		chromedp.Navigate("about:blank"),
		chromedp.EvaluateAsDevTools(postScript, nil),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
	}

	if err := chromedp.Run(taskCtx, tasks...); err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(htmlContent)),
		Request:    req,
	}, nil
}

