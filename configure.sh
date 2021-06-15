apt update

apt install emacs golang git

adduser simon

echo "export GOPATH=/home/simon/code" >> /home/simon/.profile
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

sudo curl https://raw.githubusercontent.com/brotherlogic/provisioner/master/gobuildslave.service > /etc/systemd/system/
sudo sysctl enable gobuildslave
sudo sysctl start gobuildslave 

su simon
ssh-keygen -t rsa -f /home/simon/.ssh/id_rsa -q -P ""

reboot
