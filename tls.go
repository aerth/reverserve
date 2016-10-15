package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func (s *Server) tlsServe() {

	fmt.Println("Reverserve: Serving TLS on:", *slisten)

	handle := func(w http.ResponseWriter, r *http.Request) {

		if s.targets[r.Host] == nil {
			log.Println(r.Host, "not in config")
			errar(w, r)
			return
		}
		log.Println(r.Host, "found")
		log.Println(r.URL.Scheme)
		// // Send to real proxy handler
		if r.TLS.HandshakeComplete {
			r.URL.Scheme = "https"
		}

		s.targets[r.Host].ServeHTTP(w, r)

		return
	}
	srv := &http.Server{
		Addr:           *slisten,
		Handler:        http.HandlerFunc(handle),
		ReadTimeout:    20 * time.Second,
		WriteTimeout:   20 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	z, e := tls.LoadX509KeyPair(*cert, *key)
	if e != nil {
		log.Fatalln("TLS Certificate:", e)
	} else {
		log.Println("Loaded:", *cert)
	}

	tlsListener, e := tls.Listen("tcp", srv.Addr, &tls.Config{Certificates: []tls.Certificate{z}})
	if e != nil {
		fmt.Println("tlss:", e)
		os.Exit(2)
	}
	fmt.Println("Trying:")

	tlsconf := new(tls.Config)
	for _, m := range sconfigger() {
		fmt.Println("Trying:", m[2], m[3])
		c, e := tls.LoadX509KeyPair(m[2], m[3])
		if e != nil {
			log.Fatalln("TLS Certificate:", e)
		} else {
			tlsconf.Certificates = append(tlsconf.Certificates, c)
			fmt.Println("Loaded:", m[2], m[3])
			tlsconf.ServerName = m[0]
		}
	}
	srv.Serve(tlsListener)

}
