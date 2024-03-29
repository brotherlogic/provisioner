adduser simon

apt update

apt install -y emacs golang git

echo "export GOPATH=/home/simon/go" >> /home/simon/.profile
echo "export PATH=$GOPATH/bin:$GOROOT/bin:$PATH" >> /home/simon/.profile
echo "export GOROOT=/usr/lib/go" >> /home/simon/.profile
echo "export ETCDCTL_API=3" >> /home/simon/.profile
echo "export GOPRIVATE=github.com/brotherlogic/*" >> /home/simon/.profile

echo "SHELL=/bin/bash" > /var/spool/cron/crontabs/simon
echo "MAILTO=brotherlogic@gmail.com" >> /var/spool/cron/crontabs/simon

chown simon:crontab /var/spool/cron/crontabs/simon
chmod 0600 /var/spool/cron/crontabs/simon

chmod u+w /etc/sudoers.d/010_pi-nopasswd
sed -i 's/pi/simon/g' /etc/sudoers.d/010_pi-nopasswd
chmod u-w /etc/sudoers.d/010_pi-nopasswd

sudo systemctl restart cron

curl https://raw.githubusercontent.com/brotherlogic/provisioner/master/goscript64.sh > /home/simon/goscript.sh
chmod u+x /home/simon/goscript.sh
/home/simon/goscript.sh


curl https://raw.githubusercontent.com/brotherlogic/provisioner/master/gobuildslave.service > /etc/systemd/system/gobuildslave.service
systemctl enable gobuildslave
systemctl start gobuildslave 

su simon <<EOSU
ssh-keygen -t rsa -f /home/simon/.ssh/id_rsa -q -P ""
go install github.com/brotherlogic/gobuildslave@latest
mkdir -p /home/simon/gobuild/bin
cp /home/simon/go/bin/gobuildslave /home/simon/gobuild/bin
curl https://raw.githubusercontent.com/brotherlogic/provisioner/master/gobuildslave.sh -o /home/simon/gobuild/bin/gobuildslave.sh
chmod u+x /home/simon/gobuild/bin/gobuildslave.sh

sudo reboot
EOSU