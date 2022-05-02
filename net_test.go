package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/html"
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
			panic(err)
		}
	}()

	response1, err := client.Get("http://localhost:8080/static/album_metada.json")
	if err != nil {
		panic(err)
	}
	defer response1.Body.Close()

	// start := time.Now()
	result1, err := io.ReadAll(response1.Body)
	// fmt.Println("io.ReadAll:", time.Since(start), "size: ", len(result1), cap(result1))
	if err != nil {
		panic(err)
	}

	response2, err := client.Get("http://localhost:8080/static/album_metada.json")
	if err != nil {
		panic(err)
	}
	defer response2.Body.Close()

	var lengthValue int
	if length := response2.Header.Get("Content-Length"); length != "" {
		lengthValue, err = strconv.Atoi(length)
		if err != nil {
			panic(err)
		}
	}

	// start = time.Now()
	result2, err := readAll(response2.Body, lengthValue)
	// fmt.Println("   readAll:", time.Since(start), "size: ", len(result2), cap(result2))
	if err != nil {
		t.Errorf("\nunexpected error: %v\n", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := server.Shutdown(ctx); err != nil {
		panic(err)
	}
	wg.Wait()
	cancel()

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

var testHTML = `
<!DOCTYPE html>
<html>
<head>
<title>Page Title</title>
</head>
<body>

<h1>This is a Heading</h1>
<p id="paragraph test">This is a paragraph.</p>
<ul>
	<li class="first group">
</ul>
<div id="section test">
	<ol id="ordered list">
		<li id="first entry"  class="first group" >
			<a href="https://example.com/test1.html">
				<div class="img">
					<img src="https://example.com/img1.png" />
				</div>
			</a>
		</li>
		<li id="second entry"  class="first group" >
			<a href="https://example.com/test2.html">
				<div class="img">
					<img src="https://example.com/img2.png" />
				</div>
			</a>
		</li>
		<li id="third entry"  class="first group" >
			<a href="https://example.com/test3.html">
				<div class="img">
					<img src="https://example.com/img3.png" />
				</div>
			</a>
		</li>
		<li id="fourth entry"  class="second group" >
			<a href="https://example.com/test4.html">
				<div class="img">
					<img src="https://example.com/img4.png" />
				</div>
			</a>
		</li>
		<li id="fifth entry"  class="second group" >
			<a href="https://example.com/test5.html">
				<div class="img">
					<img src="https://example.com/img5.png" />
				</div>
			</a>
		</li>
		<li id="sixth entry"  class="second group" >
			<a href="https://example.com/test6.html">
				<div class="img">
					<img src="https://example.com/img6.png" />
				</div>
			</a>
		</li>
	</ol>
</div>
<div id="empty section test"><p>Test paragraph.</p></div>
</body>
</html> 
`

// TODO: not pretty, not finished, not good
func TestGetValWithAttr(t *testing.T) {
	reader := strings.NewReader(testHTML)

	doc, err := html.Parse(reader)
	if err != nil {
		panic(err)
	}

	attr := &html.Attribute{
		Key: "class",
		Val: "first group",
	}

	want, wantOk := "first entry", true
	got, gotOk := getValWithAttr(doc, attr, "li", "id")

	if gotOk != wantOk || got != want {
		t.Errorf("\nget id attribute of first li:\nwant: \"%s\" %t\n got: \"%s\" %t", want, wantOk, got, gotOk)
	}

	attr.Key = "id"
	want, wantOk = "", false
	got, gotOk = getValWithAttr(doc, attr, "li", "id")

	if gotOk != wantOk || got != want {
		t.Errorf("\nincorrect attribute key:\nwant: \"%s\" %t\n got: \"%s\" %t", want, wantOk, got, gotOk)
	}

	attr.Key = "class"
	attr.Val = "new value"
	got, gotOk = getValWithAttr(doc, attr, "li", "id")

	if gotOk != wantOk || got != want {
		t.Errorf("\nincorrect attribute value:\nwant: \"%s\" %t\n got: \"%s\" %t", want, wantOk, got, gotOk)
	}
}

func TestGetTextWithAttr(t *testing.T) {
	reader := strings.NewReader(testHTML)

	doc, err := html.Parse(reader)
	if err != nil {
		panic(err)
	}

	attr := &html.Attribute{
		Key: "id",
		Val: "paragraph test",
	}

	want, wantOk := "This is a paragraph.", true
	got, gotOk := getTextWithAttr(doc, attr, "p")

	if gotOk != wantOk || got != want {
		t.Errorf("\nget text of tag with text:\nwant: \"%s\" %t\n got: \"%s\" %t", want, wantOk, got, gotOk)
	}

	attr.Val = "empty section test"
	want, wantOk = "", false
	got, gotOk = getTextWithAttr(doc, attr, "div")

	if gotOk != wantOk || got != want {
		t.Errorf("\nget text of tag without text:\nwant: \"%s\" %t\n got: \"%s\" %t", want, wantOk, got, gotOk)
	}
}

func TestGetValues(t *testing.T) {
	reader := strings.NewReader(testHTML)

	doc, err := html.Parse(reader)
	if err != nil {
		panic(err)
	}

	want := [4][6]string{{
		"https://example.com/img1.png",
		"https://example.com/img2.png",
		"https://example.com/img3.png",
		"https://example.com/img4.png",
		"https://example.com/img5.png",
		"https://example.com/img6.png"}, {

		"https://example.com/test1.html",
		"https://example.com/test2.html",
		"https://example.com/test3.html",
		"https://example.com/test4.html",
		"https://example.com/test5.html",
		"https://example.com/test6.html"}, {

		"first entry",
		"second entry",
		"third entry",
		"fourth entry",
		"fifth entry",
		"sixth entry"}, {

		"first group",
		"first group",
		"first group",
		"second group",
		"second group",
		"second group",
	}}

	tags := [4]string{"img", "a", "li", "li"}
	attr := [4]string{"src", "href", "id", "class"}

	var got []string
	for j := 0; j < 4; j++ {
		node, ok := getNodeWithAttr(doc, &html.Attribute{
			Key: "id",
			Val: "ordered list",
		}, "ol")
		if !ok {
			t.Fatal("\nnode not found, expected node with tag <ol>\n")
		}
		got = getValues(node, tags[j], attr[j])

		if len(got) != len(want[j]) {
			t.Fatalf("\ngot wrong number of results:\nwant: %d\n got: %d", len(want[j]), len(got))
		}

		for i := range want[j] {
			if got[i] != want[j][i] {
				t.Errorf("\nwrong result\nwant: %s\n got: %s at index %d:%d", want[j][i], got[i], j, i)
			}
		}

	}
}
