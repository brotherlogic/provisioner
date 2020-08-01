cd /home/simon
mkdir tmp
cd tmp
wget https://dl.google.com/go/go1.12.14.linux-armv6l.tar.gz
tar xzf go1.12.14.linux-armv6l.tar.gz
cd /usr/lib
sudo mkdir go-1.12
cd /usr/lib/go-1.12
sudo rsync -az /home/simon/tmp/go/* ./
cd /usr/lib
sudo rm go
sudo ln -s go-1.12 go
cd /usr/bin
sudo rm go
sudo ln -s ../lib/go/bin/go ./go

