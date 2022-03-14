package main

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type action int

const (
	actionSearch action = iota
	actionTagSearch
	actionOpen
	actionOpenURL
	actionAdd
	actionQuit
)

type arguments struct {
	action action
	path   string
	query  []string
	args   map[string]string
}

func (args *arguments) String() string {
	actions := [6]string{"search", "tagSearch", "open", "openURL", "add", "quit"}

	out := "action: " + actions[args.action]
	if args.path != "" {
		out += "; path: \"" + args.path + "\""
	}

	if args.query != nil {
		out += "; query: \"" + strings.Join(args.query, "+") + "\""
	}

	if args.args != nil {
		out += "; args: " + fmt.Sprint(args.args)
	}

	return out
}

func isValidURL(input string) (string, bool) {
	u, err := url.Parse(input)
	if err != nil {
		return "", false
	}

	// set scheme to https if input ends with ".com"
	if u.Scheme == "" && u.Host == "" && len(u.Path) > 4 {
		split := strings.Split(u.Path, "/")
		if strings.HasSuffix(split[0], ".com") {
			u.Scheme = "https"
			u.Host = split[0]
			u.Path = strings.Join(split[1:], "/")
		}
	}

	if u.Host == "" || u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}

	return u.String(), true
}

func filterSort(sort string) bool {
	return sort == "random" || sort == "date" || sort == "pop"
}

func filterFormat(format string) bool {
	return format == "cd" || format == "cassette" || format == "vinyl" || format == "all"
}

// command|query|url [optional args]
// commands:
// --quit, --exit,	-q	- quit
// --add,			-a	- add path/url to playlist
// --open,			-o	- create playlist from url/path
// <url>				- create playlist from url
// <string>				- search everywhere
// --album,			-A	- search in albums
// --track,			-T	- search in tracks
// --band,			-b	- search in artists/labels
// --user,			-u	- search in users
// --tag,			-t  - tag search
//		--sort,		-s	- sorting method
//		--format,	-f	- filter by formtat
//		--location,	-l	- location
//		--highlights	- get highlights of genre
func parseInput(input string) (*arguments, []string, error) {
	args := strings.Fields(input)
	if len(args) == 0 {
		return nil, nil, errors.New("empty input")
	}

	parsed := &arguments{}
	needValue := true
	if path, ok := isValidURL(args[0]); ok {
		parsed.action = actionOpenURL
		parsed.path = path
		return parsed, args[1:], nil
	}

	if strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "--quit", "--exit", "-q":
			parsed.action = actionQuit
			needValue = false

		case "--add", "-a":
			parsed.action = actionAdd

		case "--open", "-o":
			parsed.action = actionOpen

		case "--album", "-A":
			parsed.args = make(map[string]string, 1)
			parsed.args["item_type"] = "a"

		case "--track", "-T":
			parsed.args = make(map[string]string, 1)
			parsed.args["item_type"] = "t"

		case "--band", "-b":
			parsed.args = make(map[string]string, 1)
			parsed.args["item_type"] = "b"

		case "--user", "-u":
			parsed.args = make(map[string]string, 1)
			parsed.args["item_type"] = "f"

		case "--tag", "-t":
			parsed.action = actionTagSearch

		default:
			return nil, args, errors.New("unknown command: " + args[0])
		}
		str := args[0]
		args = args[1:]
		if len(args) == 0 && needValue {
			return nil, args, errors.New("query not specified for flag: " + str)
		}
	}

	if parsed.action == actionSearch {
		if parsed.args == nil {
			parsed.args = make(map[string]string, 1)
			parsed.args["item_type"] = ""
		}
		parsed.query = args
		args = args[len(args):]
		needValue = false
	}

	if parsed.action == actionTagSearch {
		var key string
		for len(args) > 0 {
			if strings.HasPrefix(args[0], "-") {
				if needValue {
					return nil, args, errors.New("expected value, got flag: " + args[0])
				}
				needValue = true
				switch args[0] {
				case "--tag", "-t":
					key = ""

				case "--format", "-f":
					key = "f"

				case "--location", "-l":
					key = "l"

				case "--sort", "-s":
					key = "s"

				case "--highlights":
					key = "tab"
					needValue = false

				default:
					return nil, args, errors.New("unknown flag: " + args[0])
				}

				str := args[0]
				args = args[1:]
				if len(args) == 0 && needValue {
					return nil, args, errors.New("query not specified for flag: " + str)
				}
			}

			if key == "" {
				parsed.query = append(parsed.query, args[0])
			} else {
				if parsed.args == nil {
					parsed.args = make(map[string]string, 4)
					parsed.args["tab"] = "all_releases"
				}

				if key == "l" {
					parsed.args[key] += " " + args[0]
				}

				if key == "tab" {
					parsed.args[key] = "highlights"
					return parsed, args, nil
				}

				if key == "f" {
					if ok := filterFormat(args[0]); !ok {
						return nil, args, errors.New("incorrect value for format: " + args[0])
					}
					parsed.args[key] = args[0]
				}

				if key == "s" {
					if ok := filterSort(args[0]); !ok {
						return nil, args, errors.New("incorrect value for sort: " + args[0])
					}
					parsed.args[key] = args[0]
				}
			}
			needValue = false
			args = args[1:]
		}
	}

	if needValue {
		parsed.path = strings.Join(args, " ")
		args = args[len(args):]
	}

	return parsed, args, nil
}
