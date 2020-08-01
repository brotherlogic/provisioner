package main

import (
	"bufio"
	"os/exec"

	pb "github.com/brotherlogic/provisioner/proto"
	"golang.org/x/net/context"
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

// Cluster attempt to register
func (s *Server) Cluster(ctx context.Context, req *pb.ClusterRequest) (*pb.ClusterResponse, error) {
	cmd := exec.Command("etcdctl", "cluster-health")
	_, err := s.run(cmd)

	return nil, err
}
