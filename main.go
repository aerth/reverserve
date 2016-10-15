/*
  .-. .-. .-. .-. .-. .   . . .-. .-. . . .   .-.
  |-| `-.  |  |(  |-| |   |\| |-  |(  | | |   |-|
  ` ' `-'  '  ' ' ` ' `-' ' ` `-' `-' `-' `-' ` '
	Copyright (c)  2016 aerth <aerth@riseup.net>
	MIT License

*/

// Reverserve is a minimal reverse proxy.
// If the hostname is not in config.ini , it will not be served.
package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// CLI flags
var (
	listen  = flag.String("http", ":80", "Interface and port to listen on, Example: -http=127.0.0.1:80 ")
	slisten = flag.String("https", ":443", "Interface and port to listen on, Example: -http=:443\n\tNote: When using TLS, all the targets will be served with the same CERTIFICATE. This will be fixed some time.")
	key     = flag.String("key", "", "https/SSL/TLS key.pem")
	cert    = flag.String("cert", "", "https/SSL/TLS cert.pem")
	config  = flag.String("config", "config.ini", "location of config file")
	debug   = flag.Bool("v", false, "Enable logs")
)

// Server listens on 80 and 443, acting as a router to the target[map]
type Server struct {
	Name    string
	proxy   *httputil.ReverseProxy      // default proxy , can be nil. Will show 503 error in that case.
	mutex   sync.Mutex                  // This mutex guards writing/reading the map.
	targets map[string]*Proxy           // target proxies, map[url]proxy
	config  map[string]*url.URL         // config
	sconfig map[string]*tls.Certificate // sconfig is https
}

// Proxy is a reverse proxy
type Proxy struct {
	Name    string
	proxy   *httputil.ReverseProxy
	parent  *Server
	mutex   sync.Mutex // This mutex guards writing/reading the map.
	isProxy bool
	errors  *bytes.Buffer
	log     *log.Logger
}

/*
Load the config.ini into memory

The file should look something like this:
host1.com http://realhost:8080
host3.com http://realhost:8081
host4.com http://127.0.0.1:8082
_ index.html

This example config listens for requests on host1,host2,host4,
and serves the content from port 8080, 8081, 8082
_ means "serve ./index.html for everything", otherwise we give a 503 error.

*/
func (s *Server) configger() {

	log.Println("Initializing config:", *config)
	m := map[string]*url.URL{}
	b, e := ioutil.ReadFile(*config)
	if e != nil {
		log.Fatalln(e)
		log.Fatalln("Please make " + *config + " -- here is an example:\n\n" +
			"example.com http://127.0.0.1:8080\nexample2.com http://127.0.0.1:8081\n")

	}

	lines := strings.Split(string(b), "\n") // split lines
	for _, line := range lines {
		parts := strings.Split(line, " ") // check spaces
		if len(parts) != 2 {
			parts = strings.Split(line, "\t") // and tabs
			if len(parts) != 2 {
				continue
			}
		}
		u, e := url.Parse(parts[1]) // parse the second column as a URL
		if e != nil {
			log.Println(e)
			continue
		}
		s.mutex.Lock()
		log.Println("Adding: ", parts[0], u)

		m[parts[0]] = u
		s.mutex.Unlock()
	}
	s.config = m
	for i, v := range s.config {
		log.Println("Serving:", i, v)
	}

}
func sconfigger() map[string][]string {
	m := map[string][]string{}
	b, _ := ioutil.ReadFile(*config)
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		words := strings.Split(line, " ")
		if len(words) == 4 {
			m[words[0]] = words
		}
	}
	return m
}

// Reverserve
func main() {
	flag.Parse()
	if *debug {
		log.SetFlags(log.Llongfile)
		ToDo()
	}
	s := New()
	s.configger()

	if len(s.config) < 1 {
		log.Fatalln("Please make config.ini -- here is an example:\n\n" +
			"example.com http://127.0.0.1:8080\nexample2.com http://127.0.0.1:8081\n")
	}
	if *key != "" && len(s.config) < 1 {
		log.Println("Not fatal: config has no https hosts")
	}
	go s.fresh()
	s.serve()
}

// Main serve loop
func (s *Server) serve() {
	http.Handle("/", s)
	// Message listen success
	go func() {
		select {
		case <-time.After(200 * time.Millisecond):
			log.Println("Reverserve: Serving HTTP on:", *listen)
			log.Println("Host table:")
			for i, v := range s.config {
				log.Println(i, v)
			}

		}
	}()
	log.Fatal(http.ListenAndServe(*listen, nil))

}

// ServeHTTP proxy
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Hijack proxy log
	p.mutex.Lock()
	erbuf := new(bytes.Buffer)
	tmplog := log.New(erbuf, "", log.Ltime)
	p.proxy.ErrorLog = tmplog

	// Reverse proxy
	if *debug {
		log.Println(r.Host, r.RequestURI)
	}

	p.proxy.ServeHTTP(w, r)

	// Proxy log gave us an error. Error is only non-nil if no body was written.
	if erbuf.Len() != 0 {
		er := erbuf.String()
		log.Println(er)   // log real error, user gets 502 bad gateway
		erbuf.Truncate(0) // Clear p.errors buffer
		w.Write([]byte("503 Service Unavailable\n"))
	}

	p.mutex.Unlock()

	return
}

// ServeHTTP server
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// If there is no proxy by that name, give a 503 status.
	if s.targets[r.Host] == nil {
		fmt.Printf("%q %s\n", r.Host, "Host is not in map")
		log.Println("here is the entire map:", s.targets)
		errar(w, r)
		return
	}

	log.Println(r.Host, "matches", s.targets[r.Host].Name)

	// Send to real proxy handler
	s.targets[r.Host].ServeHTTP(w, r)
	return
}

// New server with req chan
func New() *Server {
	s := new(Server)
	s.targets = map[string]*Proxy{}
	s.proxy = nil
	return s
}

// NewProxyTarget creates new proxy
func NewProxyTarget(target string) (*Server, error) {
	u, e := url.Parse(target)
	if e != nil {
		return nil, e
	}
	s := New()
	s.proxy = httputil.NewSingleHostReverseProxy(u)
	return s, nil
}

func (s *Server) newProxy(target *url.URL) *Proxy {
	p := new(Proxy)
	p.isProxy = true
	p.parent = s
	var buf = new(bytes.Buffer)
	p.errors = buf
	logger2 := log.New(p.errors, "", log.Ltime)
	p.proxy = httputil.NewSingleHostReverseProxy(target)
	p.proxy.ErrorLog = logger2
	return p
}

// Bind creates a new server, places it inside s.targets map
func (s *Server) Bind(host string, target *url.URL) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	p := s.newProxy(target)
	s.targets[host] = p
	return nil
}
func errar(w http.ResponseWriter, r *http.Request) {
	if *debug {
		log.Println("Bad:", r.Host, r.RequestURI)
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	//w.Write(errorBytes)
	w.Write([]byte(http.StatusText(http.StatusServiceUnavailable)))
}
func (s *Server) fresh() {

	for {
		// Add each config host to the map
		for i, v := range s.config {
			err := s.Bind(i, v)
			if err != nil {
				panic(err)
			}
		}

		// Load every minute
		s.configger()
		time.Sleep(1 * time.Minute)
	}
}

// ToDo things
func ToDo() {
	todolist := `



	`
	log.Println(todolist)
}
