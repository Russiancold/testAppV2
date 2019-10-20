package main

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

//read file, pass urls to out till EOF
func reader(in *os.File, out chan<- string) {
	reader := bufio.NewReader(in)
	var url string
	var err error
	for {
		url, err = reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Fatal(err)
			}
			close(out)
			return
		}
		out <- url
	}
}

// count of go's found in resource corresponding to url
type result struct {
	url string
	count int
}

// aggregate bufSize urls, spread over workersCount worker's
func fanout(bufSize, workersCount int, in <-chan string, out chan result, worker func(string, chan<- result, <-chan struct{})) {
	limit := make(chan struct{}, workersCount)
	buf := make(chan string, bufSize)

	// fill buffer
	go func() {
		for url := range in {
			buf <- url
		}
		close(buf)
	}()

	for url := range buf {
		limit <- struct{}{}
		go worker(url, out, limit)
	}
}

// count 'go' strings found on url
func worker(url string, out chan<- result, limit <-chan struct{}) {
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("error get %s: %v", url, err)
		<-limit
		return
	}

	if resp.StatusCode != 200 {
		out <- result{url, 0}
		<-limit
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error read %s's body: %v", url, err)
		<-limit
		return
	}

	out <- result{url, bytes.Count(body, []byte("go"))}
	<-limit
}