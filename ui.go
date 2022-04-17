package main

type userInterface interface {
	Run(quit chan<- struct{}, input chan<- *action)
	Update()
	DisplayMessage(*message)
	Quit()
}
