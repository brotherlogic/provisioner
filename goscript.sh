cd /home/simon
rm -rf tmp/
mkdir tmp
cd tmp
wget https://dl.google.com/go/go1.22.1.linux-armv6l.tar.gz
tar xzf go1.22.1.linux-armv6l.tar.gz
cd /usr/lib
rm -rf go-1.22.1
sudo mkdir go-1.22.1
cd /usr/lib/go-1.22.1
sudo rsync -az /home/simon/tmp/go/* ./
cd /usr/lib
sudo rm go
sudo ln -s go-1.22.1 go
cd /usr/bin
sudo rm go
sudo ln -s ../lib/go/bin/go ./go

