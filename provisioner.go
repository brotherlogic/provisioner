package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
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
			s.Log(fmt.Sprintf("Config exists"))
			return
		}
	}

	s.Log(fmt.Sprintf("Setting config"))
	f, err := os.OpenFile("/etc/default/etcd", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer f.Close()
	if _, err := f.WriteString("ETCD_UNSUPPORTED_ARCH=arm\n"); err != nil {
		log.Fatalf("%v", err)
	}

	if _, err := f.WriteString(fmt.Sprintf("ETCD_ADVERTISE_CLIENT_URLS=\"http://%v:2379\"\n", s.Registry.GetIp())); err != nil {
		log.Fatalf("%v", err)
	}
	if _, err := f.WriteString(fmt.Sprintf("ETCD_INITIAL_ADVERTISE_PEER_URLS=\"http://%v:2380\"\n", s.Registry.GetIp())); err != nil {
		log.Fatalf("%v", err)
	}
	if _, err := f.WriteString(fmt.Sprintf("ETCD_LISTEN_CLIENT_URLS=\"http://%v:2379,http://localhost:2379\"\n", s.Registry.GetIp())); err != nil {
		log.Fatalf("%v", err)
	}
	if _, err := f.WriteString(fmt.Sprintf("ETCD_LISTEN_PEER_URLS=\"http://%v:2380\"\n", s.Registry.GetIp())); err != nil {
		log.Fatalf("%v", err)
	}

	s.Log(fmt.Sprintf("Config complete"))
}

func (s *Server) validateRPI() {
	if fileExists("/home/simon/rpi_exporter") {
		s.Log(fmt.Sprintf("Not installing rpi exporter"))
		return
	}

	cmd := exec.Command("go", "get", "-u", "github.com/lukasmalkmus/rpi_exporter")
	err := cmd.Run()
	s.Log(fmt.Sprintf("Ran command: %v", err))
	time.Sleep(time.Second * 10)

	cmd = exec.Command("mv", "/root/go/bin/rpi_exporter", "/home/simon/rpi_exporter")
	err = cmd.Run()
	s.Log(fmt.Sprintf("Ran copy: %v", err))

	f, err := os.OpenFile("/var/spool/cron/crontabs/simon", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer f.Close()
	if _, err := f.WriteString("@reboot sudo /home/simon/rpi_exporter\n"); err != nil {
		log.Fatalf("%v", err)
	}

	// Restart to trigger crontab
	cmd = exec.Command("reboot")
	err = cmd.Run()
}

func (s *Server) confirmVM() {
	cmd := exec.Command("sysctl", "vm.dirty_ratio")
	b, err := cmd.Output()

	if err != nil {
		s.Log(fmt.Sprintf("Error in vm confirm: %v", err))
		return
	}

	if string(b) == "vm.dirty_ratio = 10" {
		return
	}

	s.Log(fmt.Sprintf("Setting the dirty ratio"))
	exec.Command("sysctl", "-w", "vm.dirty_ratio=10").Run()
	exec.Command("sysctl", "-w", "vm.dirty_background_ratio=5").Run()
	exec.Command("sysctl", "-p").Run()
}

func (s *Server) validateNodeExporter() {
	if fileExists("/usr/bin/prometheus-node-exporter") {
		s.Log(fmt.Sprintf("Not installing node exporter"))
		return
	}

	time.Sleep(time.Second * 10)
	cmd := exec.Command("apt", "install", "-y", "prometheus-node-exporter")
	err := cmd.Run()
	s.Log(fmt.Sprintf("Ran command: %v", err))
}

func (s *Server) validateEtcRunsOnStartup() {
	if fileExists("/etc/systemd/system/etcd2.service") {
		s.Log(fmt.Sprintf("Not enabling etcd"))
		return
	}

	time.Sleep(time.Second * 5)
	cmd := exec.Command("update-rc.d", "etcd", "enable")
	err := cmd.Run()
	s.Log(fmt.Sprintf("Updated rcd: %v", err))

	time.Sleep(time.Second * 5)
	cmd = exec.Command("/etc/init.d/etcd", "start")
	err = cmd.Run()
	s.Log(fmt.Sprintf("Running etcd: %v", err))
}

func (s *Server) installGo() {
	b, err := exec.Command("go", "version").Output()
	if err != nil {
		log.Fatalf("Unable to get output: %v", err)
	}

	elems := strings.Fields(string(b))
	if elems[2] != "go1.12.14" {
		s.Log(fmt.Sprintf("Installing new go version: '%v'", string(b)))
		err := exec.Command("curl", "https://raw.githubusercontent.com/brotherlogic/provisioner/master/goscript.sh", "-o", "/home/simon/goscript.sh").Run()
		if err != nil {
			log.Fatalf("Unable to download install script: %v", err)
		}

		err = exec.Command("chmod", "u+x", "/home/simon/goscript.sh").Run()
		if err != nil {
			log.Fatalf("Unable to chmod: %v", err)
		}

		err = exec.Command("/home/simon/goscript.sh").Run()
		if err != nil {
			log.Fatalf("Bad install: %v", err)
		}
	}
}

const (
	// ID the id of the thing
	ID = "/github.com/brotherlogic/provisioner/id"
)

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
		time.Sleep(time.Second * 5)
		//server.validateEtcConfig()
		time.Sleep(time.Second * 5)
		server.validateRPI()
		time.Sleep(time.Second * 5)
		server.validateNodeExporter()
		time.Sleep(time.Second * 5)
		server.validateEtcRunsOnStartup()
		time.Sleep(time.Second * 5)
		server.confirmVM()
		time.Sleep(time.Second * 5)
		server.installGo()

		server.Log(fmt.Sprintf("Completed provisioner run"))
	}()

	fmt.Printf("%v", server.Serve())
}
