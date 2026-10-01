package session

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

// pagePath is where the proxy serves tinymist's preview page. Chromium names
// an --app window after its URL, so a path of our own gives the window a
// class nothing else has ("chrome-127.0.0.1__note-preview-Default") for the
// window manager to match on. --class does nothing for app windows.
const pagePath = "/note-preview"

// proxy sits between the preview window and tinymist. It exists to restyle
// the page: tinymist paints its backdrop a fixed gray (it only takes VS
// Code's theme colours, which a browser never has), and that gray fills the
// window for the couple of seconds the page takes to boot. Everything else,
// including the WebSocket the page streams the document over, passes through
// untouched; the page opens that socket on its own origin, so it lands here.
type proxy struct {
	srv *http.Server
	url string // what the preview window should open
}

// startProxy serves the preview at upstream on a free local port, with the
// backdrop set to backdrop (any CSS colour). An empty backdrop leaves the
// page as tinymist drew it.
func startProxy(upstream, backdrop string) (*proxy, error) {
	target, err := url.Parse(upstream)
	if err != nil {
		return nil, fmt.Errorf("invalid preview URL %q: %w", upstream, err)
	}
	host, _, err := net.SplitHostPort(target.Host)
	if err != nil {
		host = target.Hostname()
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return nil, fmt.Errorf("starting preview proxy: %w", err)
	}

	rp := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			if r.In.URL.Path == pagePath {
				r.Out.URL.Path = "/"
				// The page is rewritten below, which needs it uncompressed.
				r.Out.Header.Del("Accept-Encoding")
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			if resp.Request.URL.Path != "/" || resp.Request.Header.Get("Upgrade") != "" {
				return nil
			}
			if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
				return nil
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return err
			}
			body = restyle(body, backdrop)
			resp.Body = io.NopCloser(bytes.NewReader(body))
			resp.ContentLength = int64(len(body))
			resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
			return nil
		},
		ErrorLog: nullLogger,
	}

	p := &proxy{
		srv: &http.Server{Handler: rp, ErrorLog: nullLogger},
		url: "http://" + ln.Addr().String() + pagePath,
	}
	go p.srv.Serve(ln)
	return p, nil
}

// restyle overrides the page's backdrop colour. The page sets the variable
// inline on <html> from script, so the override needs !important to win.
func restyle(page []byte, backdrop string) []byte {
	if backdrop == "" {
		return page
	}
	style := fmt.Sprintf("<style>:root{--typst-preview-background-color:%s !important}</style>", backdrop)
	i := bytes.Index(page, []byte("<head>"))
	if i < 0 {
		return page
	}
	i += len("<head>")
	return append(page[:i:i], append([]byte(style), page[i:]...)...)
}

func (p *proxy) close() {
	if p != nil {
		p.srv.Close()
	}
}

// nullLogger keeps the proxy quiet: the terminal belongs to the editor while
// it runs, and a dropped preview connection is not worth a garbled screen.
var nullLogger = log.New(io.Discard, "", 0)
