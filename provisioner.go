package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
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

func (s *Server) installPrometheus() {
	if fileExists("/etc/init.d/prometheus") {
		s.Log("Not installing prometheus")
		return
	}

	out, err := exec.Command("apt", "install", "-y", "prometheus").Output()
	if err != nil {
		log.Fatalf("Unable to install prometheus %v -> %v", err, string(out))
	}
}

func (s *Server) installFlac() {
	if fileExists("/usr/bin/flac") {
		s.Log("Not installing flac")
		return
	}

	out, err := exec.Command("apt", "install", "-y", "flac").Output()
	if err != nil {
		log.Fatalf("Unable to install flac %v -> %v", err, string(out))
	}
}

func (s *Server) installLsof() {
	if fileExists("/usr/bin/lsof") {
		s.Log("Not installing lsof")
		return
	}

	out, err := exec.Command("apt", "install", "-y", "lsof").Output()
	if err != nil {
		log.Fatalf("Unable to install lsof %v -> %v", err, string(out))
	}
}

func (s *Server) configurePrometheus() {
	if fileExists("/etc/prometheus/jobs.json") {
		s.Log("Not configuring prometheus")
		return
	}

	out, err := exec.Command("curl", "https://raw.githubusercontent.com/brotherlogic/provisioner/master/prometheus.yml", "-o", "/etc/prometheus/prometheus.yml").Output()
	if err != nil {
		log.Fatalf("Unable to configure prometheus %v -> %v", err, string(out))
	}

	f, err := os.OpenFile("/etc/prometheus/jobs.json", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("OPEN jobs.json %v", err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		log.Fatalf("Failed to output blanks to jobs file: %v", err)
	}
	out, err = exec.Command("chown", "simon:simon", "/etc/prometheus/jobs.json").Output()
	if err != nil {
		log.Fatalf("Unable to chown jobs files%v -> %v", err, string(out))
	}

	s.Log("Configured Prometheus")

}

func (s *Server) fixTimezone() {
	out, err := exec.Command("timedatectl").Output()
	if err != nil {
		log.Fatalf("Unable to call timeactl %v -> %v", err, string(out))
	}

	if !strings.Contains(string(out), "Los_Angeles") {
		s.Log(fmt.Sprintf("Setting timezone -> %v", string(out)))
		out, err = exec.Command("timedatectl", "set-timezone", "America/Los_Angeles").Output()
		if err != nil {
			log.Fatalf("Unable to set timezone %v -> %v", err, string(out))
		}
	}
}

func (s *Server) installGrafana() {
	if fileExists("/etc/init.d/grafana-server") {
		s.Log("Not installing grafana server")
		return
	}

	out, err := exec.Command("curl", "https://packages.grafana.com/gpg.key", "-o", "/home/simon/gpg.key").Output()
	if err != nil {
		log.Fatalf("Unable to install grafana key %v -> %v", err, string(out))
	}

	out, err = exec.Command("apt-key", "add", "/home/simon/gpg.key").Output()
	if err != nil {
		log.Fatalf("Unable to install grafana (2) %v -> %v", err, string(out))
	}

	f, err := os.OpenFile("/etc/apt/sources.list.d/grafana.list", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("OPEN CONF %v", err)
	}
	if _, err := f.WriteString("deb https://packages.grafana.com/oss/deb stable main\n"); err != nil {
		log.Fatalf("Failed to output: %v", err)
	}

	out, err = exec.Command("apt", "update").Output()
	if err != nil {
		log.Fatalf("Unable to update apt %v -> %v", err, string(out))
	}

	out, err = exec.Command("apt", "install", "-y", "grafana").Output()
	if err != nil {
		log.Fatalf("Unable to install grafana server %v -> %v", err, string(out))
	}
	out, err = exec.Command("systemctl", "enable", "grafana-server").Output()
	if err != nil {
		log.Fatalf("Unable to enable the grafana server %v -> %v", err, string(out))
	}
	out, err = exec.Command("systemctl", "start", "grafana-server").Output()
	if err != nil {
		log.Fatalf("Unable to start grafana server %v -> %v", err, string(out))
	}

}

func (s *Server) validateEtc() {
	if fileExists("/etc/init.d/etcd") {
		s.Log("Not installing etcd")
		return
	}

	s.Log("Installing etcd")

	r := &epb.ExecuteResponse{}
	for r.GetStatus() != epb.CommandStatus_COMPLETE {

		ctx, cancel := utils.ManualContext("provision-etc", time.Minute)
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

	cmd := exec.Command("go", "get", "github.com/lukasmalkmus/rpi_exporter")
	bytes, err := cmd.Output()
	s.Log(fmt.Sprintf("Ran plain go get command: %v (%v)", err, string(bytes)))
	time.Sleep(time.Second * 10)

	cmd = exec.Command("mv", "/root/go/bin/rpi_exporter", "/home/simon/rpi_exporter")
	err = cmd.Run()
	s.Log(fmt.Sprintf("Ran rpi copy copy: %v", err))

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
	// Don' install rpi-export for rdisplay
	if s.Registry.Identifier == "rdisplay" {
		return
	}

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
	if elems[2] != "go1.17.6" {
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
		s.Log(fmt.Sprintf("Not installing go (%v)", elems[2]))
	}
}

const (
	// ID the id of the thing
	ID = "/github.com/brotherlogic/provisioner/id"
)

func (s *Server) procDisk(name string, needsFormat bool, needsMount bool, disk string) {
	out, err := exec.Command("tune2fs", "-l", fmt.Sprintf("/dev/%v", name)).Output()
	if err != nil {
		log.Fatalf("Bad run of tune2fs: %v -> %v", err, name)
	}
	lines := strings.Split(string(out), "\n")
	ran := false
	for _, line := range lines {
		if strings.HasPrefix(line, "Maximum mount count") {
			ran = true
			elems := strings.Fields(line)
			count, err := strconv.Atoi(elems[3])
			if err != nil {
				log.Fatalf("Can't parse int: %v ->%v", elems, err)
			}
			s.procDiskInternal(name, needsFormat, needsMount, count != 5, disk)
		}
	}
	if !ran {
		log.Fatalf("Unable to run: %v, %v -> %v", name, disk, string(out))
	}
}

func (s *Server) procDiskInternal(name string, needsFormat bool, needsMount bool, needTuneUpdate bool, disk string) {
	s.Log(fmt.Sprintf("Working on for %v %v, with view to formatting %v and mounting %v and tune update %v", disk, name, needsFormat, needsMount, needTuneUpdate))

	if needTuneUpdate {
		b, err := exec.Command("tune2fs", "-c", "5", fmt.Sprintf("/dev/%v", name)).Output()
		if err != nil {
			log.Fatalf("Bad run of tune2fs set: %v->%v", err, string(b))
		}
	}

	if needsFormat {
		b, err := exec.Command("mkfs.ext4", fmt.Sprintf("/dev/%v", name)).Output()
		if err != nil {
			log.Fatalf("Bad format: %v -> %v", err, string(b))
		}
	}

	if needsMount {
		out, err := exec.Command("mkdir", "-v", fmt.Sprintf("/media/%v", disk)).Output()
		if err != nil {
			return
		} else {
			err = exec.Command("chown", "simon:simon", fmt.Sprintf("/media/%v", disk)).Run()
			if err != nil {
				log.Fatalf("CHOWN %v", err)
			}
			f, err := os.OpenFile("/etc/fstab", os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				log.Fatalf("OPEN FSTA %v", err)
			}

			if _, err := f.WriteString(fmt.Sprintf("/dev/%v   /media/%v  ext4  defaults,nofail,nodelalloc  1  2\n", name, disk)); err != nil {
				log.Fatalf("WRITE FSTAB ERR %v", err)
			}
			f.Close()
		}

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

		if len(fields) >= 3 && fields[len(fields)-1] == "/" {
			//Ensure the root partition gets prepped
			if strings.Contains(fields[0], "sd") {
				s.procDiskInternal(fields[0][strings.Index(fields[0], "sd"):], false, false, true, "root")
			} else {
				s.procDiskInternal(fields[0][strings.Index(fields[0], "mm"):], false, false, true, "root")
			}

		}

		// This is the WD passport drive or the samsung key drive
		if len(fields) >= 3 && fields[len(fields)-1] == "part" &&
			(fields[len(fields)-2] == "238.5G" || fields[len(fields)-2] == "239G" || fields[len(fields)-2] == "119.5G" || fields[len(fields)-2] == "120G") {
			found = true
			s.procDisk(fields[0][strings.Index(fields[0], "sd"):], len(fields) != 4, len(fields) != 5, "datastore")
		}

		// This is the smaller samsung key drive
		if len(fields) >= 3 && fields[len(fields)-1] == "part" && (fields[len(fields)-2] == "29.9G") {
			found = true
			s.Log(fmt.Sprintf("thing: %v with %v", fields[1], fields))
			if fields[1] == "vfat" {
				s.RaiseIssue("One disk needs formatting", fmt.Sprintf("Disk %v on %v needs to be fdisk'd", fields[0], s.Registry.Identifier))
				time.Sleep(time.Second * 10)
			}
			s.procDisk(fields[0][strings.Index(fields[0], "sd"):], len(fields) != 4, len(fields) != 5, "scratch")
		}

		// This is the raid disk
		if len(fields) >= 3 && fields[len(fields)-1] == "part" && (fields[len(fields)-2] == "7.3T") {
			found = true
			s.procDisk(fields[0][strings.Index(fields[0], "sd"):], len(fields) != 4, len(fields) != 5, "raid")
		}

		// This is a cdrom
		if len(fields) >= 3 && fields[len(fields)-1] == "rom" {
			if !fileExists("/usr/bin/cdparanoia") {
				s.Log("Found cd drive and installing cdparanoia")
				b, err := exec.Command("apt", "install", "-y", "cdparanoia").Output()
				if err != nil {
					log.Fatalf("Cannot install cdparanoia: %v -> %v", err, string(b))
				}
			}

			if !fileExists("/usr/bin/eject") {
				s.Log("Found cd drive and installing eject")
				b, err := exec.Command("apt", "install", "-y", "eject").Output()
				if err != nil {
					log.Fatalf("Cannot install eject: %v -> %v", err, string(b))
				}
			}
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

	if _, err := f.WriteString("dtoverlay=vc4-kms-dpi-hyperpixel4sq\n"); err != nil {
		log.Fatalf("WRITE %v", err)
	}

	if _, err := f.WriteString("dtparam=poe_fan_temp0=75000,poe_fan_temp0_hyst=5000\n"); err != nil {
		log.Fatalf("WRITE %v", err)
	}

	if _, err := f.WriteString("dtparam=poe_fan_temp1=77000,poe_fan_temp1_hyst=2000\n"); err != nil {
		log.Fatalf("WRITE %v", err)
	}

	if _, err := f.WriteString("dtparam=poe_fan_temp2=80000,poe_fan_temp0_hyst=2000\n"); err != nil {
		log.Fatalf("WRITE %v", err)
	}

	if _, err := f.WriteString("dtparam=poe_fan_temp3=85000,poe_fan_temp1_hyst=2000\n"); err != nil {
		log.Fatalf("WRITE %v", err)
	}

	err = exec.Command("reboot").Run()
	if err != nil {
		log.Fatalf("REBOOT FAILED: %v", err)
	}
}

func (s *Server) prepSwap() {
	bytes, err := exec.Command("free", "-m").Output()

	if err != nil {
		log.Fatalf("failed opening file: %s", err)
	}

	lines := strings.Split(string(bytes), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			if fields[0] == "Swap:" {
				s.Log(fmt.Sprintf("Found Swap: %v", fields[1]))
				if fields[1] != "0" {
					exec.Command("dphys-swapfile", "swapoff").Run()
					exec.Command("dphys-swapfile", "uninstall").Run()
					exec.Command("systemctl", "disable", "dphys-swapfile").Run()
				}
			}
		}
	}

	s.Log(fmt.Sprintf("No Swap adjustment needed from %v lines", len(lines)))
}

func (s *Server) gui() {
	if s.Registry.GetIdentifier() != "rdisplay" {
		return
	}

	s.setAutologin()
	s.setAutoload()
}

func (s *Server) setAutoload() {
	if fileExists("/home/simon/.config/lxsession/LXDE-pi/autostart") {
		s.Log(fmt.Sprintf("Not setting auto-login"))
		return
	}

	err := os.MkdirAll("/home/simon/.config/lxsession/LXDE-pi", 0777)
	if err != nil {
		log.Fatalf("Bad mkdir: %v", err)
	}
	f, err := os.OpenFile("/home/simon/.config/lxsession/LXDE-pi/autostart", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("OPEN CONF %v", err)
	}

	exec.Command("apt", "install", "-y", "xserver-xorg", "xinit").Run()
	exec.Command("apt", "install", "-y", "lxde-core", "lxterminal", "lxappearance").Run()
	exec.Command("apt", "install", "-y", "lightdm").Run()

	exec.Command("apt", "install", "-y", "chromium-browser").Run()
	exec.Command("apt", "install", "-y", "unclutter").Run()
	exec.Command("apt", "install", "-y", "point-rpi").Run()
	exec.Command("apt", "remove", "-y", "lxplug-volumepulse").Run()
	exec.Command("apt", "remove", "-y", "cups").Run()
	exec.Command("apt", "remove", "-y", "pulseaudio").Run()
	exec.Command("apt", "remove", "-y", "packagekit").Run()
	exec.Command("apt", "remove", "-y", "system-config-printer").Run()
	exec.Command("apt", "autoremove", "-y").Run()
	exec.Command("systemctl", "disable", "packagekit").Run()

	exec.Command("rm", "/etc/xdg/autostart/piwiz.desktop").Run()

	for _, string := range []string{"@lxpanel --profile LXDE-pi",
		"@pcmanfm --desktop --profile LXDE-pi",
		"point-rpi",
		"@xset s noblank",
		"@xset s off",
		"@xset -dpms",
		"@chromium-browser  --disable-gpu --noerrdialogs --enable-features=OverlayScrollbar --disable-infobars --kiosk file:///home/simon/index.html"} {
		if _, err := f.WriteString(string + "\n"); err != nil {
			log.Fatalf("WRITE %v", err)
		}
	}

	exec.Command("chown", "-R", "simon:simon", "/home/simon/.config").Run()

	err = exec.Command("reboot").Run()
	if err != nil {
		log.Fatalf("REBOOT FAILED: %v", err)
	}
}

func (s *Server) setAutologin() {
	if fileExists("/etc/systemd/system/getty@tty1.service.d/autologin.conf") {
		s.Log(fmt.Sprintf("Not setting auto-login"))
		return
	}

	exec.Command("systemctl", "set-default", "graphical.target").Run()

	f, err := os.OpenFile("/etc/systemd/system/getty@tty1.service.d/autologin.conf", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("OPEN CONF %v", err)
	}

	for _, string := range []string{"[Service]", "ExecStart=-/sbin/agetty --autologin simon --noclear %I $TERM"} {
		if _, err := f.WriteString(string + "\n"); err != nil {
			log.Fatalf("WRITE %v", err)
		}
	}

	err = exec.Command("reboot").Run()
	if err != nil {
		log.Fatalf("REBOOT FAILED: %v", err)
	}
}

func (s *Server) prepForEtcd() {

}

func (s *Server) prepForZsh() {

	bytes, err := exec.Command("apt", "install", "-y", "finger").Output()
	if err != nil {
		log.Fatalf("Unable to install shell finger via apt: %v (%v)", err, string(bytes))
	}

	bytes, err = exec.Command("finger", "simon").Output()
	if err != nil {
		log.Fatalf("Unable to echo shell: %v (%v)", err, string(bytes))
	}

	if strings.Contains(string(bytes), "bash") {
		s.Log("Currently set for bash, moving to zsh")

		err = exec.Command("apt", "install", "-y", "zsh").Run()
		if err != nil {
			log.Fatalf("Unable to install shell finger via apt: %v", err)
		}
		err = exec.Command("chsh", "-s", "/bin/zsh", "simon").Run()
		if err != nil {
			log.Fatalf("Unable to change shell: %v", err)
		}

		// Trigger a reboot
		err = exec.Command("reboot").Run()
		if err != nil {
			log.Fatalf("Reboot did not pass:  %v", err)
		}
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
	server.RunSudo()

	err := server.RegisterServerV2("provisioner", false, true)
	if err != nil {
		return
	}

	go func() {
		time.Sleep(time.Second * 5)
		server.validateRPI()
		time.Sleep(time.Second * 5)
		server.validateNodeExporter()
		time.Sleep(time.Second * 5)
		server.confirmVM()
		time.Sleep(time.Second * 5)
		server.installGo()
		time.Sleep(time.Second * 5)
		server.installLsof()
		time.Sleep(time.Second * 5)
		server.prepDisks()
		time.Sleep(time.Second * 5)
		server.prepPoe()
		time.Sleep(time.Second * 5)
		server.prepSwap()
		time.Sleep(time.Second * 5)
		server.gui()
		time.Sleep(time.Second * 5)
		server.prepForZsh()
		time.Sleep(time.Second * 5)
		if server.Registry.GetIdentifier() == "cd" {
			server.installFlac()
			time.Sleep(time.Second * 5)
		}
		if server.Registry.GetIdentifier() == "monitoring" {
			server.installPrometheus()
			time.Sleep(time.Second * 5)
			server.configurePrometheus()
			time.Sleep(time.Second * 5)
			server.installGrafana()
			time.Sleep(time.Second * 5)
		} else {
			server.Log(fmt.Sprintf("Skipping prometheus (%v)", server.Registry.GetIdentifier()))
			time.Sleep(time.Second * 5)
		}
		server.fixTimezone()
		time.Sleep(time.Second * 5)
		server.Log("Completed provisioner run")
	}()

	fmt.Printf("%v", server.Serve())
}
