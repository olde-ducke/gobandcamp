package main

import (
	"errors"
	"strings"
)

func open(path string) error {
	if strings.HasPrefix(path, "file://") {
		return errors.New("NOT IMPLEMENTED: local file reading")
	}
	return nil
}
