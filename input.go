package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
	"github.com/pkg/errors"
)

type textField struct {
	views.TextArea
	symbols  []rune
	sbuilder strings.Builder
	previous []rune
}

func (field *textField) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {

	case *tcell.EventKey:
		if event.Key() == tcell.KeyTab {
			window.hideInput = !window.hideInput
			field.HideCursor(window.hideInput)
			field.EnableCursor(!window.hideInput)
			if !window.hideInput {
				window.sendEvent(newMessage("enter url/command"))
			} else {
				window.sendEvent(&eventDisplayMessage{})
			}
			return true
		}

		if window.hideInput {
			return false
		}

		posX, _, _, _ := field.GetModel().GetCursor()

		switch event.Key() {
		case tcell.KeyEnter:
			parseInput(field.getText())
			field.Clear()
			window.hideInput = !window.hideInput
			field.HideCursor(window.hideInput)
			field.EnableCursor(!window.hideInput)
			return true

		case tcell.KeyUp:
			field.symbols = make([]rune, len(field.previous))
			copy(field.symbols, field.previous)
			field.SetContent(string(field.symbols))
			field.SetCursorX(len(field.symbols))
			return true

		case tcell.KeyDown:
			field.Clear()
			return true

			// NOTE: only backspace2 works on linux(? not sure)
			// only regular one works on windows
		case tcell.KeyBackspace2, tcell.KeyBackspace:
			if posX > 0 {
				posX--
				field.symbols[posX] = 0
				field.symbols = append(field.symbols[:posX],
					field.symbols[posX+1:]...)
			}
			field.SetContent(string(field.symbols))
			field.SetCursorX(posX)
			return true

		case tcell.KeyDelete:
			if posX < len(field.symbols)-1 {
				field.symbols[posX] = 0
				field.symbols = append(field.symbols[:posX],
					field.symbols[posX+1:]...)
				posX++
			}
			field.SetContent(string(field.symbols))
			return true

		case tcell.KeyRune:
			field.symbols = append(field.symbols, 0)
			copy(field.symbols[posX+1:], field.symbols[posX:])
			field.symbols[posX] = event.Rune()
			field.SetContent(string(field.symbols))
			field.SetCursorX(posX + 1)
			return true
		}
	}
	return field.TextArea.HandleEvent(event)
}

func (field *textField) getText() string {
	for i, r := range field.symbols {
		// trailing space doesn't need to be in actual input
		if i == len(field.symbols)-1 {
			break
		}
		fmt.Fprint(&field.sbuilder, string(r))
	}
	field.previous = make([]rune, len(field.symbols))
	copy(field.previous, field.symbols)
	defer field.sbuilder.Reset()
	return field.sbuilder.String()
}

func (field *textField) Clear() {
	field.SetContent(" ")
	field.symbols = make([]rune, 1)
	field.symbols[0] = ' '
	field.SetCursorX(0)
}

type arguments struct {
	tags     []string
	location []string
	sort     string
	format   Format
	flag     int
}

func parseInput(input string) {
	commands := strings.Split(input, " ")
	if strings.Contains(commands[0], "http://") || strings.Contains(commands[0], "https://") {
		wg.Add(1)
		go processMediaPage(commands[0])
		return
	} else if commands[0] == "exit" || commands[0] == "q" || commands[0] == "quit" {
		app.Quit()
		return
	} else if commands[0] != "-t" && commands[0] != "--tag" {
		window.sendEvent(newErrorMessage(errors.New("unrecognised command")))
		return
	}

	args := arguments{
		sort: "top",
		tags: []string{},
	}

	for i := 0; i < len(commands); i++ {
		if i <= len(commands)-2 && strings.HasPrefix(commands[i], "-") {
			switch commands[i] {
			case "-t", "--tag":
				args.flag = 1
			case "-l", "--location":
				args.flag = 2
			case "-s", "--sort":
				args.flag = 3
			case "-f", "--format":
				args.flag = 4
			default:
				args.flag = 0
			}
			i++
		}

		if commands[i] != "" {
			switch args.flag {
			case 1:
				args.tags = append(args.tags, commands[i])
			case 2:
				args.location = append(args.location, commands[i])
			case 3:
				switch commands[i] {
				case "top", "new", "rand":
					args.sort = commands[i]
				case "random":
					args.sort = "rand"
				case "date":
					args.sort = "date"
				case "popular", "pop":
					args.sort = "top"
				default:
					args.sort = "top"
				}
			case 4:
				// NOTE: ignore error
				args.format, _ = FormatFromString(commands[i])
				// do not include t-shirts
				if args.format == TShirts {
					args.format = All
				}
			}
		}
	}

	wg.Add(1)
	go processTagPage(args)
}

// initialize widget
func init() {
	textField := &textField{}
	textField.Clear()
	textField.previous = make([]rune, 1)
	textField.previous[0] = ' '
	window.widgets[field] = textField
}
