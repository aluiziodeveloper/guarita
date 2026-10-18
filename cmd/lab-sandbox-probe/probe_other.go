//go:build !linux

package main

import "errors"

var errUnsupported = errors.New("só no Linux")

func keyring() error { return errUnsupported }

func setuidRoot() error { return errUnsupported }
