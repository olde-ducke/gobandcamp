package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sync"
)

type headless struct {
	wg sync.WaitGroup
}

func (h *headless) run(quit chan<- struct{}) {
	h.wg.Add(1)
	go h.start()
	h.wg.Wait()
	defer func() { quit <- struct{}{} }()
}

func (h *headless) update() {
	fmt.Print("\r" + player.getStatus().String() + "\r")
}

func (h *headless) displayMessage(message string) {
	log.Println(message)
}

func (h *headless) start() {
	var input string
	scanner := bufio.NewScanner(os.Stdin)
loop:
	for scanner.Scan() {
		input = scanner.Text()
		switch input {
		case "q":
			break loop
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println(err)
	}
	h.wg.Done()
}

func newHeadless() ui {
	return &headless{}
}
