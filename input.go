package main

import (
	"errors"
	"net/url"
	"strings"
)

var (
	tagLink    = url.URL{Scheme: "https", Host: "bandcamp.com", Path: "/tag/"}
	searchLink = url.URL{Scheme: "https", Host: "bandcamp.com", Path: "/search"}
)

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

func checkSort(sort string) bool {
	return sort == "random" || sort == "date" || sort == "pop"
}

func checkFormat(format string) bool {
	return format == "cd" || format == "cassette" || format == "vinyl" || format == "all"
}

func createTagSearchURL(query []string, params map[string]string) (string, error) {
	if len(query) == 0 {
		return "", errors.New("empty query")
	}

	// scheme://host/tag/<tag-name>?tab=<tab_name>&t=<other-tag>&s=sort&f=<format>&l=<int>

	u := tagLink
	u.Path += strings.ToLower(query[0])
	v := url.Values{}

	for key, val := range params {
		v.Set(key, val)
	}

	if len(query) > 1 {
		v.Set("t", strings.Join(query[1:], ","))
	}

	u.RawQuery = v.Encode()

	return u.String(), nil
}

func createSearchURL(query []string, params map[string]string) (string, error) {
	if len(query) == 0 {
		return "", errors.New("empty query")
	}

	u := searchLink
	v := url.Values{}

	v.Set("q", strings.Join(query, " "))
	for key, val := range params {
		v.Set(key, val)
	}

	u.RawQuery = v.Encode()

	return u.String(), nil
}

// [command] query|path [args]
// commands:
// --quit, --exit   -q  - quit
// --add			-a  - add path/url to playlist
// --open           -o  - create playlist from url/path
// <url>				- create playlist from url
// <string>				- search everywhere
// --album          -A  - search in albums
// --track          -T  - search in tracks
// --band           -b  - search in artists/labels
// --user           -u  - search in users
// --tag            -t  - tag search
// tag arguments:
// --sort           -s  - sorting method
// --format	        -f  - filter by formtat
// --location	    -l  - location
// --highlights        	- get highlights of genre
func parseInput(input string) (*action, []string, error) {
	if len(input) == 0 {
		return nil, nil, errors.New("empty input")
	}

	args := strings.Fields(input)
	parsed := &action{}
	if path, ok := isValidURL(args[0]); ok {
		parsed.actionType = actionOpenURL
		parsed.path = path
		return parsed, args[1:], nil
	}

	query := make([]string, 0)
	params := make(map[string]string, 0)

	if strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "--quit", "--exit", "-q":
			parsed.actionType = actionQuit
			return parsed, args[1:], nil

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

		args = args[1:]
		if len(args) == 0 {
			return nil, args, errors.New("query not specified for flag: " + args[0])
		}
	}

	if parsed.actionType == actionSearch {
		url, err := createSearchURL(args, params)
		if err != nil {
			return nil, nil, err
		}
		parsed.path = url
		return parsed, nil, nil
	}

	if parsed.actionType != actionTagSearch {
		parsed.path = strings.Join(args, " ")
		return parsed, nil, nil
	}

	needValue := true
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
			if key == "l" {
				params[key] += " " + args[0]
			}

			if key == "tab" {
				params[key] = "highlights"
				return parsed, args, nil
			}

			if key == "f" {
				if ok := checkFormat(args[0]); !ok {
					return nil, args, errors.New("incorrect value for format: " + args[0])
				}
				params[key] = args[0]
			}

			if key == "s" {
				if ok := checkSort(args[0]); !ok {
					return nil, args, errors.New("incorrect value for sort: " + args[0])
				}
				params[key] = args[0]
			}
		}

		needValue = false
		args = args[1:]
	}

	params["tab"] = "all_releases"
	url, err := createTagSearchURL(query, params)
	if err != nil {
		return nil, nil, err
	}
	parsed.path = url
	return parsed, args, nil
}
