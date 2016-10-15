package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aerth/filer"
	"github.com/stretchr/testify/assert"
)

/*
Testing that:
		config loads map[host]URL
		if request hostOne, give hostOne content
		hostOne does not interfere with hostTwo
		if hostTwo is unpublished, it is not leaked
		finally, hostOne through hostFour get smashed (...think Gallagher)

*/

func t1Handler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ONE\n"))
}
func t2Handler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("TWO\n"))
}
func t3Handler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("THREE\n"))
}
func t4Handler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("FOUR\n"))
}

// func TestMain(m *testing.M) {
// 	flag.Parse()
// 	os.Exit(m.Run())
// }

var ts0, ts1, ts2, ts3, ts4 *httptest.Server
var s *Server

func init() {
	fmt.Println("Test Init")
	*listen = ":9989"

	ts1 = httptest.NewServer(http.HandlerFunc(t1Handler))
	ts2 = httptest.NewServer(http.HandlerFunc(t2Handler))
	ts3 = httptest.NewServer(http.HandlerFunc(t3Handler))
	ts4 = httptest.NewServer(http.HandlerFunc(t4Handler))
	filer.Touch("/tmp/testingConfig1")
	filer.Write("/tmp/testingConfig1", []byte(""))
	filer.Append("/tmp/testingConfig1", []byte("testOne.com\t"+ts1.URL+"\n"))
	filer.Append("/tmp/testingConfig1", []byte("testTwo.com\t"+ts2.URL+"\n"))
	filer.Append("/tmp/testingConfig1", []byte("testThree.com\t"+ts3.URL+"\n"))
	filer.Append("/tmp/testingConfig1", []byte("testFour.com\t"+ts4.URL+"\n"))
	//	go s.fresh() // TestFresh later..
	*config = "/tmp/testingConfig1"
	s = New()
	s.configger()
	go s.fresh()
	ts0 = httptest.NewServer(s)
}

func TestConfig(t *testing.T) {
	if len(s.config) < 1 {
		fmt.Println("Unexpected 'no config'")
		t.Fail()
	}
}

func TestHostOne(t *testing.T) {
	// sleep 100ms to give the goroutines a chance
	time.Sleep(100 * time.Millisecond)
	// check for testOne.com match
	for i := 0; i < 5; i++ {
		logbuf := new(bytes.Buffer)
		log.SetOutput(logbuf)

		req, e := http.NewRequest("GET", "testOne.com", nil)
		assert.Nil(t, e)
		req.Host = "testOne.com"
		w := httptest.NewRecorder()
		f := s
		f.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)

		assert.Equal(t, "ONE\n", w.Body.String())
		if w.Body.String() != "ONE\n" {
			fmt.Println(w.Body.String())
		}

		assert.True(t, strings.Contains(logbuf.String(), "testOne.com matches"))
	}
}

func TestHostTwo(t *testing.T) {
	// sleep 100ms to give the goroutines a chance
	time.Sleep(100 * time.Millisecond)
	// check for testOne.com match
	for i := 0; i < 5; i++ {
		logbuf := new(bytes.Buffer)
		log.SetOutput(logbuf)

		req, e := http.NewRequest("GET", "testTwo.com", nil)
		assert.Nil(t, e)
		req.Host = "testTwo.com"
		w := httptest.NewRecorder()
		f := s
		f.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)

		assert.Equal(t, "TWO\n", w.Body.String())
		if w.Body.String() != "TWO\n" {
			fmt.Println(w.Body.String())
		}

		assert.True(t, strings.Contains(logbuf.String(), "testTwo.com matches"))
	}
}
func TestHostThree(t *testing.T) {
	// sleep 100ms to give the goroutines a chance
	time.Sleep(100 * time.Millisecond)
	// check for testOne.com match
	for i := 0; i < 5; i++ {
		logbuf := new(bytes.Buffer)
		log.SetOutput(logbuf)

		req, e := http.NewRequest("GET", "testThree.com", nil)
		assert.Nil(t, e)
		req.Host = "testThree.com"
		w := httptest.NewRecorder()
		f := s
		f.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)

		assert.Equal(t, "THREE\n", w.Body.String())
		if w.Body.String() != "THREE\n" {
			fmt.Println(w.Body.String())
		}

		assert.True(t, strings.Contains(logbuf.String(), "testThree.com matches"))
	}
}

func TestHostFour(t *testing.T) {
	// sleep 100ms to give the goroutines a chance
	time.Sleep(100 * time.Millisecond)
	// check for testOne.com match
	for i := 0; i < 5; i++ {
		logbuf := new(bytes.Buffer)
		log.SetOutput(logbuf)

		req, e := http.NewRequest("GET", "testFour.com", nil)
		assert.Nil(t, e)
		req.Host = "testFour.com"
		w := httptest.NewRecorder()
		f := s
		f.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)

		assert.Equal(t, "FOUR\n", w.Body.String())
		if w.Body.String() != "FOUR\n" {
			fmt.Println(w.Body.String())
		}

		assert.True(t, strings.Contains(logbuf.String(), "testFour.com matches"))
	}
}

// Smash the server with random requests
func TestSmash(t *testing.T) {
	fmt.Println("This test is costs 4 puppies.")
	time.Sleep(1 * time.Second)
	fmt.Println("And takes 5 seconds")
	time.Sleep(1 * time.Second)
	var wg sync.WaitGroup
	t1 := time.Now()
	for u := 0; u < 2500; u++ { // Hit 10,000 URLs in 5 seconds
		//for u := 0; u < 10000; u++ { // Hit 40,000 URLs in 16.71 seconds
		for hostname := range s.config {
			wg.Add(1)
			go func(i string) {
				req, e := http.NewRequest("GET", i, nil)
				assert.Nil(t, e)
				req.Host = i
				w := httptest.NewRecorder()
				f := s
				f.ServeHTTP(w, req)
				assert.Equal(t, 200, w.Code)
				bod, _ := ioutil.ReadAll(w.Result().Body)
				switch string(bod) {
				case "ONE":
					assert.Equal(t, "testOne.com", i)
				case "TWO":
					assert.Equal(t, "testTwo.com", i)
				case "THREE":
					assert.Equal(t, "testThree.com", i)
				case "FOUR":
					assert.Equal(t, "testFour.com", i)
				}
				if !t.Failed() {
					fmt.Println("Head:", i, "PASS")
				}
				wg.Done()
			}(hostname)
		}

		wg.Wait()
		if !t.Failed() {
			fmt.Println("Smash Test", "PASS", time.Now().Sub(t1).String())
		}
	}
}
