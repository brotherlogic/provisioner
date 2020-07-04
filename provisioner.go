package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/goserver/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	epb "github.com/brotherlogic/executor/proto"
	pbg "github.com/brotherlogic/goserver/proto"
)

//Server main server type
type Server struct {
	*goserver.GoServer
}

// Init builds the server
func Init() *Server {
	s := &Server{
		GoServer: &goserver.GoServer{},
	}
	return s
}

// DoRegister does RPC registration
func (s *Server) DoRegister(server *grpc.Server) {

}

// ReportHealth alerts if we're not healthy
func (s *Server) ReportHealth() bool {
	return true
}

// Shutdown the server
func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}

// Mote promotes/demotes this server
func (s *Server) Mote(ctx context.Context, master bool) error {
	return nil
}

// GetState gets the state of the server
func (s *Server) GetState() []*pbg.State {
	return []*pbg.State{
		&pbg.State{Key: "magic", Value: int64(13)},
	}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func (s *Server) validateEtc() {
	if fileExists("/etc/init.d/etcd") {
		s.Log("Not installing etcd")
		return
	}

	s.Log("Installing etcd")

	r := &epb.ExecuteResponse{}
	for r.GetStatus() != epb.CommandStatus_COMPLETE {

		ctx, cancel := utils.ManualContext("provision-etc", "provision-etc", time.Minute, true)
		defer cancel()

		conn, err := s.FDialSpecificServer(ctx, "executor", s.Registry.Identifier)
		if err != nil {
			log.Fatalf("Unable to dial executor: %v", err)
		}
		defer conn.Close()

		client := epb.NewExecutorServiceClient(conn)
		r, err := client.QueueExecute(ctx, &epb.ExecuteRequest{Command: &epb.Command{Binary: "sudo", Parameters: []string{"apt", "install", "-y", "etcd"}}})
		if err != nil {
			s.Log(fmt.Sprintf("Unable to run execute: %v", err))
		}

		time.Sleep(time.Second)
		s.Log(fmt.Sprintf("Result %v", r))
	}

	s.Log("Installed etcd")
}

func (s *Server) validateEtcConfig() {
	file, err := os.Open("/etc/default/etcd")
	defer file.Close()

	if err != nil {
		log.Fatalf("failed opening file: %s", err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		if scanner.Text() == "ETCD_UNSUPPORTED_ARCH=arm" {
			return
		}
	}

	f, err := os.OpenFile("text.log", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer f.Close()
	if _, err := f.WriteString("ETCD_UNSUPPORTED_ARCH=arm\n"); err != nil {
		log.Fatalf("%v", err)
	}
}

func main() {
	var quiet = flag.Bool("quiet", false, "Show all output")
	flag.Parse()

	//Turn off logging
	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}
	server := Init()
	server.PrepServer()
	server.Register = server

	err := server.RegisterServerV2("provisioner", false, true)
	if err != nil {
		return
	}

	go func() {
		time.Sleep(time.Second * 5)
		server.validateEtc()
	}()

	fmt.Printf("%v", server.Serve())
}
