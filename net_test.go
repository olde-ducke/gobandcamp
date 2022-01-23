package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestReadAllAlphabet(t *testing.T) {
	abc := `abcdefghijklmnopqrstuvwxyz`

	r := bytes.NewReader([]byte(abc))
	size := len(abc)

	for i := 0; i <= 2; i++ {
		result, err := readAll(r, size)
		if err != nil {
			t.Errorf("\nunexpected error: %v\n", err)
		}

		if string(result) != abc {
			t.Errorf("\nwrong result:\nwant: %s\n got: %s\n", abc, string(result))
		}

		if len(result) != len(abc) {
			t.Errorf("\nwrong length:\nwant: len(%d)\n got: len(%d)\n",
				len(abc), len(result))

		}

		if i == 0 && cap(result) != len(abc) {
			t.Errorf("\nwrong capacity:\nwant: cap(%d)\n got: cap(%d)\n",
				len(abc), cap(result))
		} else if i > 0 && cap(result) == len(abc) {
			t.Errorf("\nwrong capacity:\nwant: cap(>%d)\n got: cap(%d)\n",
				len(abc), cap(result))
		}
		r.Seek(0, 0)
		size = 1

		if i == 1 {
			size = 33
		}
	}
}

func TestReadAllAgainstReal(t *testing.T) {
	var wg sync.WaitGroup

	server := &http.Server{Addr: ":8080"}
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./testdata"))))

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	response1, err := client.Get("http://localhost:8080/static/album_metada.json")
	if err != nil {
		log.Fatal(err)
	}
	defer response1.Body.Close()

	start := time.Now()
	result1, err := io.ReadAll(response1.Body)
	fmt.Println("io.ReadAll:", time.Since(start), "size: ", len(result1), cap(result1))
	if err != nil {
		log.Fatal(err)
	}

	response2, err := client.Get("http://localhost:8080/static/album_metada.json")
	if err != nil {
		log.Fatal(err)
	}
	defer response2.Body.Close()

	var lengthValue int
	if length := response2.Header.Get("Content-Length"); length != "" {
		lengthValue, err = strconv.Atoi(length)
		if err != nil {
			log.Fatal(err)
		}
	}

	start = time.Now()
	result2, err := readAll(response2.Body, lengthValue)
	fmt.Println("   readAll:", time.Since(start), "size: ", len(result2), cap(result2))
	if err != nil {
		t.Errorf("\nunexpected error: %v\n", err)
	}

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
	wg.Wait()

	if len(result1) != len(result2) {
		t.Errorf("\nwrong length:\nwant: len(%d)\n got: len(%d)\n",
			len(result1), len(result2))

	}

	if len(result1) != cap(result2) {
		t.Errorf("\nwrong capacity:\nwant: len(%d)\n got: len(%d)\n",
			len(result1), cap(result2))
	}

	for i, b := range result1 {
		if b != result2[i] {
			t.Errorf("\nwrong data:\nwant: %x\n got: %x at index %d\n", b, result2[i], i)
		}
	}
}
