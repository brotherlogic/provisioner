package main

import (
	"log"
	"os/exec"
	"testing"
)

func TestRunCommand(t *testing.T) {
	s := Init()

	cmd := exec.Command("ls")
	output, err := s.run(cmd)

	log.Printf("Ran: [%v] %v -> %v", len(output), output, err)
}
