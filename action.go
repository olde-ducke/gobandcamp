package main

type actionType int

const (
	actionSearch actionType = iota
	actionTagSearch
	actionOpen
	actionOpenURL
	actionAdd
	actionStart
	actionPlay
	actionDownload
	actionQuit
)

type action struct {
	actionType actionType
	path       string
}
