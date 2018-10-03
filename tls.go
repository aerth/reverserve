package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func (s *Server) servetls(slisten string, cert, key string) {

	fmt.Println("Reverserve: Serving TLS on:", slisten)
	var handle http.HandlerFunc
	if len(s.targets) == 0 {
		handle = http.NotFound
	} else {
		handle = func(w http.ResponseWriter, r *http.Request) {
			log.Println(r.Method, r.Host, r.URL.Path, r.RemoteAddr, r.UserAgent(), r.ContentLength)
			if s.targets[r.Host] == nil {
				log.Println(r.Host, "not in config, denying")
				errar(w, r)
				return
			}
			if r.TLS.HandshakeComplete {
				r.URL.Scheme = "https"
			}

			// // Send to real proxy handler
			s.targets[r.Host].ServeHTTP(w, r)

			return
		}
	}
	srv := &http.Server{
		Addr:           slisten,
		Handler:        http.HandlerFunc(handle),
		ReadTimeout:    20 * time.Second,
		WriteTimeout:   20 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	z, e := tls.LoadX509KeyPair(cert, key)
	if e != nil {
		log.Fatalln("TLS Certificate:", e)
	} else {
		log.Println("Loaded:", cert)
	}

	tlsListener, e := tls.Listen("tcp", srv.Addr, &tls.Config{Certificates: []tls.Certificate{z}})
	if e != nil {
		fmt.Println("tlss:", e)
		os.Exit(2)
	}
	fmt.Println("Trying:")

	tlsconf := new(tls.Config)
	for _, m := range file2map("config.ini") {
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
