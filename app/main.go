// Default echo backend: returns request metadata as JSON, in the same
// shape as httpbin.org/anything. Used as default_app in the gateway's
// CDS/LDS - the catch-all cluster any route without a more specific
// backend falls through to.
package main

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
)

// maxBodyBytes caps how much of a request body echo will buffer, so a
// large/unbounded body can't exhaust memory - this backend is reachable
// directly on the gateway's public port.
const maxBodyBytes = 10 << 20 // 10MiB

type response struct {
	Args    map[string]string `json:"args"`
	Data    string            `json:"data"`
	Headers map[string]string `json:"headers"`
	Method  string            `json:"method"`
	Origin  string            `json:"origin"`
	URL     string            `json:"url"`
}

const banner = `{"app":"refract","message":"Relax, refract it"}`

// root serves the JSON banner for GET/HEAD on exactly "/", and echoes
// everything else (other paths, other methods) so default_app's
// catch-all behavior is unchanged.
func root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, banner)
		return
	}
	echo(w, r)
}

func echo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, _ := io.ReadAll(r.Body)

	args := map[string]string{}
	for key, values := range r.URL.Query() {
		args[key] = values[0]
	}

	headers := map[string]string{}
	for key, values := range r.Header {
		headers[key] = values[0]
	}

	origin := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		origin = host
	}

	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS != nil {
		scheme = "https"
	}
	url := scheme + "://" + r.Host + r.URL.RequestURI()

	data := response{
		Args:    args,
		Data:    string(body),
		Headers: headers,
		Method:  r.Method,
		Origin:  origin,
		URL:     url,
	}

	log.Printf("[app] %+v", data)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func main() {
	port := "5050"
	if len(os.Args) > 1 {
		port = os.Args[1]
	} else if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	http.HandleFunc("/", root)
	addr := "0.0.0.0:" + port
	log.Printf("HTTP server listening on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
