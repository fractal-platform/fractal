#!/bin/bash
set -ex

# will be replaced by travis
VERSION=__VERSION__

# prepare folder
mkdir -p ~/fractal-test
cd ~/fractal-test

function download() {
	filename=$1
	fileurl="https://github.com/fractal-platform/fractal/releases/download/$VERSION/$filename"

    printf "Downloading package from $fileurl\\n"

	rm -f $filename
	curl -L -O $fileurl

	if [ "$?" != "0" ];then
    		printf "\\n\\tDownload packages failed. Exiting now.\\n\\n"
    		exit 1
	fi

	tar zxvf $filename
}

OS_NAME=$( cat /etc/os-release | grep ^NAME | cut -d'=' -f2 | sed 's/\"//gI' )

case "$OS_NAME" in
  "Ubuntu")
     echo "installing fractal apps in Ubuntu Linux"
     filename=fractal-bin.$VERSION.ubuntu.tgz
     download $filename
     cp fractal-bin/gftl /usr/local/bin/
     cp fractal-bin/gtool /usr/local/bin/
     cp fractal-bin/libwasmlib.so /usr/lib/
     ;;
  *)
     printf "\\n\\tUnsupported Linux Distribution. Exiting now.\\n\\n"
     exit 1
esac

# check path
if [[ $PATH != *"/usr/local/bin"* ]]; then
    export PATH=/usr/local/bin:$PATH
    printf "\\nYou need to set your PATH enviroment var:\\n"
    printf "\\texport PATH=/usr/local/bin:\$PATH\\n\\n"
fi

# check version
gftl --help | grep -A1 VERSION
if [ "$?" != "0" ];then
    printf "\\n\\tGet fractal bin version failed. Exiting now.\\n\\n"
    exit 1
fi

printf "\\n\\tInstall fractal success.\\n\\n"
exit 0

