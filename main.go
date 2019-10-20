package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

const (
	bufSize = 10
	workersCount = 5
)

func main() {
	results := make(chan result)
	read := make(chan string)
	go reader(os.Stdin, read)
	go fanout(bufSize, workersCount, read, results, worker)
	fanin(results)
}

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
		out <- strings.TrimSpace(url)
	}
}

// count of go's found in resource corresponding to url
type result struct {
	url string
	count int
}

// aggregate bufSize urls, spread over workersCount worker's
func fanout(bufSize, workersCount int, in <-chan string, out chan result, worker func(string, chan<- result, <-chan struct{}, *sync.WaitGroup)) {
	limit := make(chan struct{}, workersCount)
	buf := make(chan string, bufSize)

	// fill buffer
	go func() {
		for url := range in {
			buf <- url
		}
		close(buf)
	}()

	wg := &sync.WaitGroup{}
	for url := range buf {
		limit <- struct{}{}
		wg.Add(1)
		go worker(url, out, limit, wg)
	}
	wg.Wait()
	close(out)
}

// count 'go' strings found on url
func worker(url string, out chan<- result, limit <-chan struct{}, wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		<-limit
	}()

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("error get %s: %v", url, err)
		return
	}

	if resp.StatusCode != 200 {
		log.Printf("got: %d from %s", resp.StatusCode, url)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error read %s's body: %v", url, err)
		return
	}

	out <- result{url, bytes.Count(body, []byte("go"))}
}

// aggregates results, prints them at the end
func fanin(in <-chan result) {
	var results []result
	for res := range in {
		results = append(results, res)
	}

	var total int
	for i := range results {
		fmt.Printf("Count for %s: %d\n", results[i].url, results[i].count)
		total += results[i].count
	}
	fmt.Print("Total:", total)
}
