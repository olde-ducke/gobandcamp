package main

type userInterface interface {
	Run(quit chan<- struct{}, input chan<- string)
	Update()
	DisplayMessage(*message)
	Quit()
}
