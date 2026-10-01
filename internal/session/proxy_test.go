package session

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const fakePage = "<!doctype html>\n<html>\n<head>\n<title>n.typ</title>\n</head>\n<body></body>\n</html>\n"

// fakeTinymist serves a preview page at "/" and answers WebSocket upgrades
// there the way tinymist does, then echoes one line.
func fakeTinymist(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Header.Get("Upgrade") == "websocket":
			conn, rw, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			defer conn.Close()
			rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
			rw.Flush()
			line, _ := rw.ReadString('\n')
			rw.WriteString("echo " + line)
			rw.Flush()
		case r.URL.Path == "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, fakePage)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestProxyPaintsTheBackdrop(t *testing.T) {
	up := fakeTinymist(t)
	p, err := startProxy(up.URL, "#191724")
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()

	if !strings.HasSuffix(p.url, pagePath) {
		t.Errorf("proxy URL %q does not end in %s", p.url, pagePath)
	}
	code, body := get(t, p.url)
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	want := "<head><style>:root{--typst-preview-background-color:#191724 !important}</style>"
	if !strings.Contains(body, want) {
		t.Errorf("page was not restyled:\n%s", body)
	}
	if !strings.Contains(body, "<title>n.typ</title>") {
		t.Errorf("page lost its content:\n%s", body)
	}
}

func TestProxyLeavesThePageAloneWithoutABackdrop(t *testing.T) {
	up := fakeTinymist(t)
	p, err := startProxy(up.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()

	if _, body := get(t, p.url); body != fakePage {
		t.Errorf("page changed:\n%s", body)
	}
}

// The page streams the document over a WebSocket on its own origin, which is
// the proxy; the upgrade has to reach tinymist intact.
func TestProxyPassesTheWebSocketThrough(t *testing.T) {
	up := fakeTinymist(t)
	p, err := startProxy(up.URL, "#191724")
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()

	addr, _ := hostPort(p.url)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	io.WriteString(conn, "GET / HTTP/1.1\r\nHost: "+addr+"\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"+
		"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n")
	r := bufio.NewReader(conn)
	resp, err := http.ReadResponse(r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("upgrade got status %d", resp.StatusCode)
	}
	io.WriteString(conn, "hello\n")
	if line, _ := r.ReadString('\n'); line != "echo hello\n" {
		t.Errorf("socket got %q", line)
	}
}

func TestProxyForwardsOtherPaths(t *testing.T) {
	up := fakeTinymist(t)
	p, err := startProxy(up.URL, "#191724")
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()

	base := strings.TrimSuffix(p.url, pagePath)
	if code, _ := get(t, base+"/missing"); code != http.StatusNotFound {
		t.Errorf("other path got status %d, want tinymist's 404", code)
	}
}
