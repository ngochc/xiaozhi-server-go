# Installing Xiaozhi Server (Go) on CentOS 8

## 1. Download and Install Go (Version 1.24+)

```bash
cd /tmp
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
```

Set environment variables:

```bash
echo 'export PATH=/usr/local/go/bin:$PATH' >> ~/.bashrc
source ~/.bashrc
```

To make changes permanent system-wide, edit `/etc/profile`:

```bash
vim /etc/profile
```

Add to the bottom:

```bash
export GO111MODULE=on
export GOROOT=/usr/local/go
export GOPATH=/home/gopath
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin
```

Reload environment variables:

```bash
source /etc/profile
```

Verify the Go version:

```bash
go version
```

(Optional) Configure Go proxy if needed:

```bash
go env -w GOPROXY=https://proxy.golang.org,direct
```

## 2. Optional: Opus Libraries

The project now includes pure-Go Opus decoding by default. If building components against external C libopus, ensure version 1.3.1+ is installed:

### Install build dependencies

```bash
sudo dnf install gcc make autoconf automake libtool
```

### Build and install latest Opus

```bash
cd /tmp
git clone https://github.com/xiph/opus.git
cd opus
./autogen.sh
./configure
make -j$(nproc)
sudo make install
export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig
```

Add to `.bashrc`:

```bash
echo 'export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig' >> ~/.bashrc
source ~/.bashrc
```

## 3. Clone Repository

```bash
git clone https://github.com/AnimeAIChat/xiaozhi-server-go
cd xiaozhi-server-go
```

Copy `config.yaml` to `.config.yaml` and configure your API keys and credentials:

```bash
cp config.yaml .config.yaml
```

## 4. Run the Server

```bash
go mod tidy
go run ./src/main.go
```
