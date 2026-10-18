package main

import (
	"errors"
	"testing"
)

// O gerador só fala com o loopback, nos dois papéis.
func TestLoad_soLoopback(t *testing.T) {
	if err := load("http://10.0.0.1:8080/mcp", 1, 1); !errors.Is(err,
		errNotLoopback) {
		t.Errorf("load: %v", err)
	}
	if err := serveFake("0.0.0.0:9001"); !errors.Is(err, errNotLoopback) {
		t.Errorf("serveFake: %v", err)
	}
}
