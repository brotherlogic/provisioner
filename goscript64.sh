cd /home/simon
rm -rf tmp/
mkdir tmp
cd tmp
wget https://dl.google.com/go/go1.20.1.linux-arm64.tar.gz
tar xzf go1.20.1.linux-arm64.tar.gz
cd /usr/lib
rm -rf go-1.20
sudo mkdir go-1.20
cd /usr/lib/go-1.20
sudo rsync -az /home/simon/tmp/go/* ./
cd /usr/lib
sudo rm go
sudo ln -s go-1.20 go
cd /usr/bin
sudo rm go
sudo ln -s ../lib/go/bin/go ./go

