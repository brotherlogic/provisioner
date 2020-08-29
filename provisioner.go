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
		log.Fatalf("OPEN CONF %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString("ETCD_UNSUPPORTED_ARCH=arm\n"); err != nil {
		log.Fatalf("WRITE %v", err)
	}

	if _, err := f.WriteString(fmt.Sprintf("ETCD_ADVERTISE_CLIENT_URLS=\"http://%v:2379\"\n", s.Registry.GetIp())); err != nil {
		log.Fatalf("ADD1 %v", err)
	}
	if _, err := f.WriteString(fmt.Sprintf("ETCD_INITIAL_ADVERTISE_PEER_URLS=\"http://%v:2380\"\n", s.Registry.GetIp())); err != nil {
		log.Fatalf("ADD2 %v", err)
	}
	if _, err := f.WriteString(fmt.Sprintf("ETCD_LISTEN_CLIENT_URLS=\"http://%v:2379,http://localhost:2379\"\n", s.Registry.GetIp())); err != nil {
		log.Fatalf("ADD3 %v", err)
	}
	if _, err := f.WriteString(fmt.Sprintf("ETCD_LISTEN_PEER_URLS=\"http://%v:2380\"\n", s.Registry.GetIp())); err != nil {
		log.Fatalf("ADD4 %v", err)
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
		log.Fatalf("CRR %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString("@reboot sudo /home/simon/rpi_exporter\n"); err != nil {
		log.Fatalf("WRCR %v", err)
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
	if elems[2] != "go1.13.14" {
		s.Log(fmt.Sprintf("Installing new go version: '%v'", string(b)))
		err := exec.Command("curl", "https://raw.githubusercontent.com/brotherlogic/provisioner/master/goscript.sh", "-o", "/home/simon/goscript.sh").Run()
		if err != nil {
			log.Fatalf("Unable to download install script: %v", err)
		}

		err = exec.Command("chmod", "u+x", "/home/simon/goscript.sh").Run()
		if err != nil {
			log.Fatalf("Unable to chmod: %v", err)
		}

		err = exec.Command("bash", "/home/simon/goscript.sh").Run()
		if err != nil {
			log.Fatalf("Bad install: %v", err)
		}
	} else {
		s.Log(fmt.Sprintf("Not installing go"))
	}
}

const (
	// ID the id of the thing
	ID = "/github.com/brotherlogic/provisioner/id"
)

func (s *Server) procDatastoreDisk(name string, needsFormat bool, needsMount bool) {
	s.Log(fmt.Sprintf("Working on %v, with view to formatting %v and mounting %v", name, needsFormat, needsMount))

	if needsFormat {
		b, err := exec.Command("mkfs.ext4", fmt.Sprintf("/dev/%v", name)).Output()
		if err != nil {
			log.Fatalf("Bad format: %v -> %v", err, string(b))
		}
	}

	if needsMount {
		err := exec.Command("mkdir", "/media/datastore").Run()
		if err != nil {
			log.Fatalf("MKDIR: %v", err)
		}
		err = exec.Command("chown", "simon:simon", "/media/datastore").Run()
		if err != nil {
			log.Fatalf("CHOWN %v", err)
		}
		f, err := os.OpenFile("/etc/fstab", os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("OPEN FSTAB %v", err)
		}
		defer f.Close()
		if _, err := f.WriteString(fmt.Sprintf("/dev/%v   /media/datastore  ext4  defaults,nofail,nodelalloc  1  2\n", name)); err != nil {
			log.Fatalf("WRITE FSTAB %v", err)
		}

		err = exec.Command("mount", "/media/datastore").Run()
		if err != nil {
			log.Fatalf("MOUNT %v", err)
		}
	}
}

func (s *Server) procDisk(name string, needsFormat bool, needsMount bool, disk string) {
	s.Log(fmt.Sprintf("Working on for scratch %v, with view to formatting %v and mounting %v", name, needsFormat, needsMount))

	if needsFormat {
		b, err := exec.Command("mkfs.ext4", fmt.Sprintf("/dev/%v", name)).Output()
		if err != nil {
			log.Fatalf("Bad format: %v -> %v", err, string(b))
		}
	}

	if needsMount {
		out, err := exec.Command("mkdir", "-v", fmt.Sprintf("/media/%v", disk)).Output()
		if err != nil {
			str := string(err.(*exec.ExitError).Stderr)
			log.Fatalf("MKDIR %v %v -> %v, %v", fmt.Sprintf("/media/%v", disk), err, out, str)
		}
		err = exec.Command("chown", "simon:simon", fmt.Sprintf("/media/%v", disk)).Run()
		if err != nil {
			log.Fatalf("CHOWN %v", err)
		}
		f, err := os.OpenFile("/etc/fstab", os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("OPEN FSTA %v", err)
		}

		if _, err := f.WriteString(fmt.Sprintf("/dev/%v   /media/%v  ext4  defaults,nofail,nodelalloc  1  2\n", name, disk)); err != nil {
			log.Fatalf("WRITE FST %v", err)
		}
		f.Close()

		out, err = exec.Command("mount", fmt.Sprintf("/media/%v", disk)).Output()
		if err != nil {
			str := string(err.(*exec.ExitError).Stderr)
			log.Fatalf("MOUNT %v -> %v, %v", err, out, str)
		}

		err = exec.Command("chown", "simon:simon", fmt.Sprintf("/media/%v", disk)).Run()
		if err != nil {
			log.Fatalf("CHOWN %v", err)
		}

	}
}

func (s *Server) prepDisks() {
	b, err := exec.Command("lsblk", "-o", "NAME,FSTYPE,SIZE,TYPE,MOUNTPOINT").Output()
	if err != nil {
		log.Fatalf("Bad run of lsblk: %v", err)
	}

	found := false
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)

		// This is the WD passport drive or the samsung key drive
		if len(fields) >= 3 && fields[len(fields)-1] == "part" && (fields[len(fields)-2] == "238.5G" || fields[len(fields)-2] == "239G") {
			found = true
			s.procDisk(fields[0][strings.Index(fields[0], "sd"):], len(fields) != 4, len(fields) != 5, "datastore")
		}

		// This is the smaller samsung key drive
		if len(fields) >= 3 && fields[len(fields)-1] == "part" && (fields[len(fields)-2] == "29.9G") {
			found = true
			s.procDisk(fields[0][strings.Index(fields[0], "sd"):], len(fields) != 4, len(fields) != 5, "scratch")
		}

		// This is the raid disk
		if len(fields) >= 3 && fields[len(fields)-1] == "part" && (fields[len(fields)-2] == "7.3T") {
			found = true
			s.procDisk(fields[0][strings.Index(fields[0], "sd"):], len(fields) != 4, len(fields) != 5, "raid")
		}
	}

	if !found {
		s.Log(fmt.Sprintf("No disk found"))
	}
}

func (s *Server) prepPoe() {
	file, err := os.Open("/boot/config.txt")
	defer file.Close()

	if err != nil {
		log.Fatalf("failed opening file: %s", err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "dtparam=poe_fan_temp0") {
			s.Log(fmt.Sprintf("Found poe settings"))
			return
		}
	}

	s.Log(fmt.Sprintf("Setting poe settings"))

	f, err := os.OpenFile("/boot/config.txt", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("OPEN CONF %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString("dtoverlay=rpi-poe\n"); err != nil {
		log.Fatalf("WRITE %v", err)
	}

	if _, err := f.WriteString("dtparam=poe_fan_temp0=65000,poe_fan_temp0_hyst=5000\n"); err != nil {
		log.Fatalf("WRITE %v", err)
	}

	if _, err := f.WriteString("dtparam=poe_fan_temp1=67000,poe_fan_temp1_hyst=2000\n"); err != nil {
		log.Fatalf("WRITE %v", err)
	}

	if _, err := f.WriteString("dtparam=poe_fan_temp2=70000,poe_fan_temp0_hyst=2000\n"); err != nil {
		log.Fatalf("WRITE %v", err)
	}

	if _, err := f.WriteString("dtparam=poe_fan_temp3=75000,poe_fan_temp1_hyst=2000\n"); err != nil {
		log.Fatalf("WRITE %v", err)
	}

	err = exec.Command("reboot").Run()
	if err != nil {
		log.Fatalf("REBOOT FAILED: %v", err)
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
	server.DiskLog = true

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
		time.Sleep(time.Second * 5)
		server.prepDisks()
		time.Sleep(time.Second * 5)
		cancel, err := server.Elect()
		if err != nil {
			server.RaiseIssue(fmt.Sprintf("%v cannot elect", server.Registry.GetIdentifier()), fmt.Sprintf("Reason: %v", err))
		}
		cancel()
		time.Sleep(time.Second * 5)
		server.prepPoe()
		time.Sleep(time.Second * 5)
		server.Log(fmt.Sprintf("Completed provisioner run"))
	}()

	fmt.Printf("%v", server.Serve())
}
