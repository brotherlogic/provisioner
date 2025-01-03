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
	"sync"
	"time"

	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/goserver/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	epb "github.com/brotherlogic/executor/proto"
	pbg "github.com/brotherlogic/goserver/proto"
)

// Server main server type
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

func (s *Server) installPrometheus(ctx context.Context) {
	if fileExists("/etc/init.d/prometheus") {
		s.CtxLog(ctx, "Not installing prometheus")
		return
	}

	out, err := exec.Command("apt", "install", "-y", "prometheus").Output()
	if err != nil {
		log.Fatalf("Unable to install prometheus %v -> %v", err, string(out))
	}
}

func (s *Server) installFlac(ctx context.Context) {
	if fileExists("/usr/bin/flac") {
		s.CtxLog(ctx, "Not installing flac")
		return
	}

	out, err := exec.Command("apt", "install", "-y", "flac").Output()
	if err != nil {
		log.Fatalf("Unable to install flac %v -> %v", err, string(out))
	}
}

func (s *Server) installLsof(ctx context.Context) {
	if fileExists("/usr/bin/lsof") {
		s.CtxLog(ctx, "Not installing lsof")
		return
	}

	out, err := exec.Command("apt", "install", "-y", "lsof").Output()
	if err != nil {
		log.Fatalf("Unable to install lsof %v -> %v", err, string(out))
	}
}

func (s *Server) configurePrometheus(ctx context.Context) {
	if fileExists("/etc/prometheus/jobs.json") {
		s.CtxLog(ctx, "Not configuring prometheus")
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

	s.CtxLog(ctx, "Configured Prometheus")

}

func (s *Server) fixTimezone(ctx context.Context) {
	out, err := exec.Command("timedatectl").Output()
	if err != nil {
		log.Fatalf("Unable to call timeactl %v -> %v", err, string(out))
	}

	if !strings.Contains(string(out), "Los_Angeles") {
		s.CtxLog(ctx, fmt.Sprintf("Setting timezone -> %v", string(out)))
		out, err = exec.Command("timedatectl", "set-timezone", "America/Los_Angeles").Output()
		if err != nil {
			log.Fatalf("Unable to set timezone %v -> %v", err, string(out))
		}
	}
}

func (s *Server) installGrafana(ctx context.Context) {
	if fileExists("/etc/init.d/grafana-server") {
		s.CtxLog(ctx, "Not installing grafana server")
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

func (s *Server) validateEtc(ctx context.Context) {
	if fileExists("/etc/init.d/etcd") {
		s.CtxLog(ctx, "Not installing etcd")
		return
	}

	s.CtxLog(ctx, "Installing etcd")

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
			s.CtxLog(ctx, fmt.Sprintf("Unable to run execute: %v", err))
		}

		time.Sleep(time.Second)
		s.CtxLog(ctx, fmt.Sprintf("Result %v", r))
	}

	s.CtxLog(ctx, "Installed etcd")
}

func (s *Server) validateEtcConfig(ctx context.Context) {
	file, err := os.Open("/etc/default/etcd")
	defer file.Close()

	if err != nil {
		log.Fatalf("failed opening file: %s", err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		if scanner.Text() == "ETCD_UNSUPPORTED_ARCH=arm" {
			s.CtxLog(ctx, fmt.Sprintf("Config exists"))
			return
		}
	}

	s.CtxLog(ctx, fmt.Sprintf("Setting config"))
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

	s.CtxLog(ctx, fmt.Sprintf("Config complete"))
}

func (s *Server) validateRPI(ctx context.Context) {
	if fileExists("/home/simon/rpi_exporter") {
		s.CtxLog(ctx, fmt.Sprintf("Not installing rpi exporter"))
		return
	}

	cmd := exec.Command("go", "install", "github.com/lukasmalkmus/rpi_exporter@latest")
	bytes, err := cmd.Output()
	s.CtxLog(ctx, fmt.Sprintf("Ran plain go get command: %v (%v)", err, string(bytes)))
	time.Sleep(time.Second * 10)

	cmd = exec.Command("mv", "/root/go/bin/rpi_exporter", "/home/simon/rpi_exporter")
	err = cmd.Run()
	s.CtxLog(ctx, fmt.Sprintf("Ran rpi copy copy: %v", err))

	f, err := os.OpenFile("/var/spool/cron/crontabs/simon", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("CRR %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString("@reboot sudo /home/simon/rpi_exporter\n"); err != nil {
		log.Fatalf("WRCR %v", err)
	}
	if strings.Contains(s.Registry.Identifier, "display") {
		if _, err := f.WriteString("0 * * * * sudo /etc/init.d.lightdm restart\n"); err != nil {
			log.Fatalf("WRCR %v", err)
		}
	}

	// Restart to trigger crontab
	cmd = exec.Command("reboot")
	err = cmd.Run()
}

func (s *Server) confirmVM(ctx context.Context) {
	cmd := exec.Command("sysctl", "vm.dirty_ratio")
	b, err := cmd.Output()

	if err != nil {
		s.CtxLog(ctx, fmt.Sprintf("Error in vm confirm: %v", err))
		return
	}

	if string(b) == "vm.dirty_ratio = 10" {
		return
	}

	s.CtxLog(ctx, fmt.Sprintf("Setting the dirty ratio"))
	exec.Command("sysctl", "-w", "vm.dirty_ratio=10").Run()
	exec.Command("sysctl", "-w", "vm.dirty_background_ratio=5").Run()
	exec.Command("sysctl", "-p").Run()
}

func (s *Server) validateNodeExporter(ctx context.Context) {
	if fileExists("/usr/bin/prometheus-node-exporter") {
		s.CtxLog(ctx, fmt.Sprintf("Not installing node exporter"))
		return
	}

	time.Sleep(time.Second * 10)
	cmd := exec.Command("apt", "install", "-y", "prometheus-node-exporter")
	err := cmd.Run()
	s.CtxLog(ctx, fmt.Sprintf("Ran command: %v", err))
}

func (s *Server) validateEtcRunsOnStartup(ctx context.Context) {
	if fileExists("/etc/systemd/system/etcd2.service") {
		s.CtxLog(ctx, fmt.Sprintf("Not enabling etcd"))
		return
	}

	time.Sleep(time.Second * 5)
	cmd := exec.Command("update-rc.d", "etcd", "enable")
	err := cmd.Run()
	s.CtxLog(ctx, fmt.Sprintf("Updated rcd: %v", err))

	time.Sleep(time.Second * 5)
	cmd = exec.Command("/etc/init.d/etcd", "start")
	err = cmd.Run()
	s.CtxLog(ctx, fmt.Sprintf("Running etcd: %v", err))
}

func (s *Server) installGo(ctx context.Context) {
	b, err := exec.Command("go", "version").Output()
	if err != nil {
		log.Fatalf("Unable to get output: %v", err)
	}

	elems := strings.Fields(string(b))
	if elems[2] != "go1.23.4" {
		s.CtxLog(ctx, fmt.Sprintf("Installing new go version: '%v'", string(b)))
		if s.Bits == 64 {
			err := exec.Command("curl", "https://raw.githubusercontent.com/brotherlogic/provisioner/master/goscript64.sh", "-o", "/home/simon/goscript.sh").Run()
			if err != nil {
				log.Fatalf("Unable to download install script: %v", err)
			}
		} else {
			err := exec.Command("curl", "https://raw.githubusercontent.com/brotherlogic/provisioner/master/goscript.sh", "-o", "/home/simon/goscript.sh").Run()
			if err != nil {
				log.Fatalf("Unable to download install script: %v", err)
			}
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
		s.CtxLog(ctx, fmt.Sprintf("Not installing go (%v)", elems[2]))
	}
}

const (
	// ID the id of the thing
	ID = "/github.com/brotherlogic/provisioner/id"
)

func (s *Server) procDisk(ctx context.Context, name string, needsFormat bool, needsMount bool, disk string) {
	if needsFormat {
		s.procDiskInternal(ctx, name, needsFormat, needsMount, true, disk)
	}
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
			s.procDiskInternal(ctx, name, needsFormat, needsMount, count != 5, disk)
		}
	}
	if !ran {
		log.Fatalf("Unable to run: %v, %v -> %v", name, disk, string(out))
	}
}

func (s *Server) procDiskInternal(ctx context.Context, name string, needsFormat bool, needsMount bool, needTuneUpdate bool, disk string) {
	s.CtxLog(ctx, fmt.Sprintf("Working on for %v %v, with view to formatting %v and mounting %v and tune update %v", disk, name, needsFormat, needsMount, needTuneUpdate))

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

		if needTuneUpdate {
			b, err := exec.Command("tune2fs", "-c", "5", fmt.Sprintf("/dev/%v", name)).Output()
			if err != nil {
				log.Fatalf("Bad run of tune2fs set: %v->%v", err, string(b))
			}
		}

	}
}

func (s *Server) prepDisks(ctx context.Context) {
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
				s.procDiskInternal(ctx, fields[0][strings.Index(fields[0], "sd"):], false, false, true, "root")
			} else {
				s.procDiskInternal(ctx, fields[0][strings.Index(fields[0], "mm"):], false, false, true, "root")
			}

		}

		if len(fields) >= 3 && fields[len(fields)-1] == "part" && fields[len(fields)-2] == "238.4G" {
			found = true
			s.procDisk(ctx, fields[0][strings.Index(fields[0], "sd"):], len(fields) != 4, len(fields) != 5, "datastore")
		}

		// This is the WD passport drive or the samsung key drive
		if len(fields) >= 3 && fields[len(fields)-1] == "part" &&
			(fields[len(fields)-2] == "238.5G" || fields[len(fields)-2] == "239G" || fields[len(fields)-2] == "119.5G" || fields[len(fields)-2] == "120G") {
			found = true
			s.procDisk(ctx, fields[0][strings.Index(fields[0], "sd"):], len(fields) != 4, len(fields) != 5, "datastore")
		}

		// This is the smaller samsung key drive
		if len(fields) >= 3 && fields[len(fields)-1] == "part" && (fields[len(fields)-2] == "29.9G") {
			found = true
			s.CtxLog(ctx, fmt.Sprintf("thing: %v with %v", fields[1], fields))
			if fields[1] == "vfat" {
				s.RaiseIssue("One disk needs formatting", fmt.Sprintf("Disk %v on %v needs to be fdisk'd", fields[0], s.Registry.Identifier))
				time.Sleep(time.Second * 10)
			}
			s.procDisk(ctx, fields[0][strings.Index(fields[0], "sd"):], len(fields) != 4, len(fields) != 5, "scratch")
		}

		// This is the raid disk
		if len(fields) >= 3 && fields[len(fields)-1] == "part" && (fields[len(fields)-2] == "7.3T") {
			found = true
			s.procDisk(ctx, fields[0][strings.Index(fields[0], "sd"):], len(fields) != 4, len(fields) != 5, "raid")
		}

		// This is a cdrom
		if len(fields) >= 3 && fields[len(fields)-1] == "rom" {
			if !fileExists("/usr/bin/cdparanoia") {
				s.CtxLog(ctx, "Found cd drive and installing cdparanoia")
				b, err := exec.Command("apt", "install", "-y", "cdparanoia").Output()
				if err != nil {
					log.Fatalf("Cannot install cdparanoia: %v -> %v", err, string(b))
				}
			}

			if !fileExists("/usr/bin/eject") {
				s.CtxLog(ctx, "Found cd drive and installing eject")
				b, err := exec.Command("apt", "install", "-y", "eject").Output()
				if err != nil {
					log.Fatalf("Cannot install eject: %v -> %v", err, string(b))
				}
			}
		}
	}

	if !found {
		s.CtxLog(ctx, fmt.Sprintf("No disk found"))
	}
}

func (s *Server) prepPoe(ctx context.Context) {
	file, err := os.Open("/boot/config.txt")
	defer file.Close()

	if err != nil {
		log.Fatalf("failed opening file: %s", err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "dtparam=poe_fan_temp0") {
			s.CtxLog(ctx, fmt.Sprintf("Found poe settings"))
			return
		}
	}

	s.CtxLog(ctx, fmt.Sprintf("Setting poe settings"))

	f, err := os.OpenFile("/boot/config.txt", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("OPEN CONF %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString("dtoverlay=rpi-poe\n"); err != nil {
		log.Fatalf("WRITE %v", err)
	}

	if s.Registry.Identifier == "rdisplay" {
		if _, err := f.WriteString("dtoverlay=vc4-kms-dpi-hyperpixel4sq\n"); err != nil {
			log.Fatalf("WRITE %v", err)
		}
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

func (s *Server) prepSwap(ctx context.Context) {
	bytes, err := exec.Command("free", "-m").Output()

	if err != nil {
		log.Fatalf("failed opening file: %s", err)
	}

	lines := strings.Split(string(bytes), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			if fields[0] == "Swap:" {
				s.CtxLog(ctx, fmt.Sprintf("Found Swap: %v", fields[1]))
				if fields[1] != "0" {
					exec.Command("dphys-swapfile", "swapoff").Run()
					exec.Command("dphys-swapfile", "uninstall").Run()
					exec.Command("systemctl", "disable", "dphys-swapfile").Run()
				}
			}
		}
	}

	s.CtxLog(ctx, fmt.Sprintf("No Swap adjustment needed from %v lines", len(lines)))
}

func (s *Server) gui(ctx context.Context) {
	if !strings.Contains(s.Registry.GetIdentifier(), "display") {
		return
	}

	s.setAutologin(ctx)
	s.setAutoload(ctx)
}

func (s *Server) setAutoload(ctx context.Context) {
	if fileExists("/home/simon/.config/lxsession/LXDE-pi/autostart") {
		s.CtxLog(ctx, fmt.Sprintf("Not setting auto-login"))
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

	exec.Command("apt", "install", "-y", "rpd-plym-splash").Run()
	exec.Command("apt", "install", "-y", "xserver-xorg", "xinit").Run()
	exec.Command("apt", "install", "-y", "raspberrypi-ui-mods").Run()
	exec.Command("apt", "install", "-y", "lightdm").Run()

	s.RaiseIssue("Setup boot", "Setup boot for user")

	exec.Command("apt", "install", "-y", "chromium-browser", "rpi-chromium-mods").Run()
	exec.Command("apt", "install", "-y", "unclutter").Run()
	exec.Command("apt", "install", "-y", "point-rpi").Run()
	exec.Command("apt", "remove", "-y", "lxplug-volumepulse").Run()
	exec.Command("apt", "purge", "-y", "cups").Run()
	exec.Command("apt", "purge", "-y", "pulseaudio").Run()
	exec.Command("apt", "purge", "-y", "packagekit").Run()
	exec.Command("apt", "purge", "-y", "system-config-printer").Run()
	exec.Command("apt", "purge", "-y", "light-locker").Run()
	exec.Command("apt", "purge", "-y", "cups-browsed").Run()
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

func (s *Server) setAutologin(ctx context.Context) {
	if fileExists("/etc/systemd/system/getty@tty1.service.d/autologin.conf") {
		s.CtxLog(ctx, fmt.Sprintf("Not setting auto-login"))
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

func (s *Server) prepForEtcd(ctx context.Context) {

}

func (s *Server) prepForDocker(ctx context.Context) {
	if s.Registry.Identifier != "clust6" && s.Registry.Identifier != "clust3" && s.Registry.Identifier != "clust7" {
		return
	}

	// which exits with status code 1 if not found
	_, err := exec.Command("which", "docker").CombinedOutput()
	if err != nil {
		_, err := exec.Command("curl", "https://get.docker.com", "-o", "/home/simon/docker-install.sh").CombinedOutput()
		if err != nil {
			log.Fatalf("Unable to download docker install script")
		}

		_, err = exec.Command("sh", "/home/simon/docker-install.sh").CombinedOutput()
		if err != nil {
			log.Fatalf("Unable to install docker: %v", err)
		}

		s.CtxLog(ctx, "Docker installed")
	}
}

func (s *Server) prepForMongo(ctx context.Context) {
	if s.Registry.Identifier != "skipper" {
		return
	}

	// Only install if mongo is not installed
	_, err := exec.Command("mongo", "--version").CombinedOutput()
	if err == nil {
		//return
	}

	out, err := exec.Command("wget", "https://www.mongodb.org/static/pgp/server-4.4.asc", "-O", "/home/simon/server-4.4.asc").Output()
	if err != nil {
		log.Fatalf("Unable to download mongo key %v -> %v", err, string(out))
	}

	out, err = exec.Command("apt-key", "add", "/home/simon/server-4.4.asc").CombinedOutput()
	if err != nil {
		log.Fatalf("Unable to install mongo  %v -> %v", err, string(out))
	}

	f, err := os.OpenFile("/etc/apt/sources.list.d/mongodb-org-4.4.list", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("OPEN CONF %v", err)
	}
	if _, err := f.WriteString("deb [ arch=amd64,arm64 ] https://repo.mongodb.org/apt/ubuntu focal/mongodb-org/4.4 multiverse\n"); err != nil {
		log.Fatalf("Failed to output: %v", err)
	}

	bytes, err := exec.Command("apt", "update").CombinedOutput()
	if err != nil {
		log.Fatalf("Unable to update apt: %v (%v)", err, string(bytes))
	}

	bytes, err = exec.Command("apt", "install", "mongodb-org", "-y").CombinedOutput()
	if err != nil {
		log.Fatalf("Unable to install mongo : %v (%v)", err, string(bytes))
	}

	bytes, err = exec.Command("sed", "-i", "s:/var/lib/mongodb:/media/mongo/:g", "/etc/mongod.conf").CombinedOutput()
	if err != nil {
		log.Fatalf("Unable to run sed: %v (%v)", err, string(bytes))
	}

	bytes, err = exec.Command("chown", "-R", "mongodb:mongodb", "/media/mongo").CombinedOutput()
	if err != nil {
		log.Fatalf("Unable to run chown: %v (%v)", err, string(bytes))
	}
}

func (s *Server) prepForZsh(ctx context.Context) {

	bytes, err := exec.Command("apt", "install", "-y", "finger").Output()
	if err != nil {
		log.Fatalf("Unable to install shell finger via apt: %v (%v)", err, string(bytes))
	}

	bytes, err = exec.Command("finger", "simon").Output()
	if err != nil {
		log.Fatalf("Unable to echo shell: %v (%v)", err, string(bytes))
	}

	if strings.Contains(string(bytes), "bash") {
		s.CtxLog(ctx, "Currently set for bash, moving to zsh")

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
	server.PrepServer("provisioner")
	server.Register = server
	server.DiskLog = true
	server.RunSudo()

	err := server.RegisterServerV2(false)
	if err != nil {
		return
	}

	ctx, cancel := utils.ManualContext("provisioner-init", time.Hour)
	defer cancel()

	swg := &sync.WaitGroup{}
	swg.Add(1)
	go func() {
		time.Sleep(time.Second * 5)
		server.validateRPI(ctx)
		time.Sleep(time.Second * 5)
		server.validateNodeExporter(ctx)
		time.Sleep(time.Second * 5)
		server.confirmVM(ctx)
		time.Sleep(time.Second * 5)
		server.installGo(ctx)
		time.Sleep(time.Second * 5)
		server.installLsof(ctx)
		time.Sleep(time.Second * 5)
		server.prepDisks(ctx)
		time.Sleep(time.Second * 5)
		server.prepPoe(ctx)
		time.Sleep(time.Second * 5)
		server.prepSwap(ctx)
		time.Sleep(time.Second * 5)
		server.prepForDocker(ctx)
		time.Sleep(time.Second * 5)
		server.gui(ctx)
		time.Sleep(time.Second * 5)
		server.prepForZsh(ctx)
		time.Sleep(time.Second * 5)
		if server.Registry.GetIdentifier() == "cd" {
			server.installFlac(ctx)
			time.Sleep(time.Second * 5)
		}
		if server.Registry.GetIdentifier() == "monitoring" {
			server.installPrometheus(ctx)
			time.Sleep(time.Second * 5)
			server.configurePrometheus(ctx)
			time.Sleep(time.Second * 5)
			server.installGrafana(ctx)
			time.Sleep(time.Second * 5)
		} else {
			server.CtxLog(ctx, fmt.Sprintf("Skipping prometheus (%v)", server.Registry.GetIdentifier()))
			time.Sleep(time.Second * 5)
		}
		server.prepForMongo(ctx)
		time.Sleep(time.Second * 5)
		server.fixTimezone(ctx)
		time.Sleep(time.Minute * 5)
		server.CtxLog(ctx, "Completed provisioner run")

		swg.Done()
	}()

	if server.Registry.Identifier == "clust3" {
		server.CtxLog(ctx, "Waiting for sync group")
		swg.Wait()
		server.CtxLog(ctx, "Wait complete")
	}

	fmt.Printf("%v", server.Serve())
}
