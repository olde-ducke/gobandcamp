package main

import (
	"errors"
	"net/url"
	"strings"
)

type actionType int

const (
	actionSearch actionType = iota
	actionTagSearch
	actionOpen
	actionOpenURL
	actionAdd
	actionStart
	actionQuit
)

type action struct {
	actionType actionType
	path       string
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

// [command] query|path [args]
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
func parseInput(input string) (*action, []string, error) {
	if len(input) == 0 {
		return nil, nil, errors.New("empty input")
	}

	args := strings.Fields(input)

	parsed := &action{}
	needValue := true
	if path, ok := isValidURL(args[0]); ok {
		parsed.actionType = actionOpenURL
		parsed.path = path
		return parsed, args[1:], nil
	}

	var query = make([]string, 0)
	var params = make(map[string]string, 0)

	if strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "--quit", "--exit", "-q":
			parsed.actionType = actionQuit
			needValue = false

		case "--add", "-a":
			parsed.actionType = actionAdd

		case "--open", "-o":
			parsed.actionType = actionOpen

		case "--album", "-A":
			params["item_type"] = "a"

		case "--track", "-T":
			params["item_type"] = "t"

		case "--band", "-b":
			params["item_type"] = "b"

		case "--user", "-u":
			params["item_type"] = "f"

		case "--tag", "-t":
			parsed.actionType = actionTagSearch

		default:
			return nil, args, errors.New("unknown command: " + args[0])
		}

		flag := args[0]
		args = args[1:]
		if len(args) == 0 && needValue {
			return nil, args, errors.New("query not specified for flag: " + flag)
		}
	}

	if parsed.actionType == actionSearch {
		if params == nil {
			params = make(map[string]string, 1)
			params["item_type"] = ""
		}
		query = args
		args = args[len(args):]
		needValue = false
	}

	if parsed.actionType == actionTagSearch {
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

				flag := args[0]
				args = args[1:]
				if len(args) == 0 && needValue {
					return nil, args, errors.New("value not specified for flag: " + flag)
				}
			}

			if key == "" {
				query = append(query, args[0])
			} else {
				if params == nil {
					params = make(map[string]string, 4)
					params["tab"] = "all_releases"
				}

				if key == "l" {
					params[key] += " " + args[0]
				}

				if key == "tab" {
					params[key] = "highlights"
					return parsed, args, nil
				}

				if key == "f" {
					if ok := filterFormat(args[0]); !ok {
						return nil, args, errors.New("incorrect value for format: " + args[0])
					}
					params[key] = args[0]
				}

				if key == "s" {
					if ok := filterSort(args[0]); !ok {
						return nil, args, errors.New("incorrect value for sort: " + args[0])
					}
					params[key] = args[0]
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

func createTagSearchURL(query []string, params map[string]string) (string, error) {
	return "", nil
}

func createSearchURL(query []string, params map[string]string) (string, error) {
	return "", nil
}
