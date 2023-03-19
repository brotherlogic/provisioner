package main

import (
	"bufio"
	"os/exec"
)

func (s *Server) run(cmd *exec.Cmd) ([]string, error) {
	out, _ := cmd.StdoutPipe()
	ack := make(chan bool)
	output := make([]string, 0)
	if out != nil {
		scanner := bufio.NewScanner(out)
		go func() {
			for scanner != nil && scanner.Scan() {
				output = append(output, scanner.Text())
			}
			out.Close()
			ack <- true
		}()
	}

	err := cmd.Run()
	<-ack
	return output, err
}
