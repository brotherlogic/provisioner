apt update

apt install emacs golang git

adduser simon

echo "export GOPATH=/home/simon/code" >> /home/simon/.profile
echo "export PATH=$GOPATH/bin:$GOROOT/bin:$PATH" >> /home/simon/.profile
echo "export GOROOT=/usr/lib/go" >> /home/simon/.profile

echo "SHELL=/bin/bash" > /var/spool/cron/crontabs/simon
echo "MAILTO=brotherlogic@gmail.com" >> /var/spool/cron/crontabs/simon
echo "0 * * * * source /home/simon/.profile; go get -u github.com/brotherlogic/gobuildslave &> /home/simon/goget" >> /var/spool/cron/crontabs/simon
echo "*/5 * * * * source /home/simon/.profile; cd /home/simon/code/src/github.com/brotherlogic/gobuildslave; python BuildAndRun.py &>> out.txt" >> /var/spool/cron/crontabs/simon

chown simon:crontab /var/spool/cron/crontabs/simon
chmod 0600 /var/spool/cron/crontabs/simon

chmod u+w /etc/sudoers.d/010_pi-nopasswd
sed -i 's/pi/simon/g' /etc/sudoers.d/010_pi-nopasswd
chmod u-w /etc/sudoers.d/010_pi-nopasswd

sudo systemctl restart cron

reboot
