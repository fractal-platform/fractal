#!/bin/sh

# prepare folder
mkdir -p ~/fractal-test
cd ~/fractal-test

function download() {
	filename=$1
	fileurl="https://github.com/fractal-platform/fractal/releases/download/v0.2.0/$filename"

	rm -f $filename
	wget $fileurl

	if [ "$?" != "0" ];then
    		printf "\\n\\tDownload packages failed. Exiting now.\\n\\n"
    		exit 1
	fi

	tar zxvf $filename
}

unamestr=`uname`
if [[ "${unamestr}" == 'Darwin' ]]; then
    echo "installing fractal apps in MacOS"
    filename=fractal-bin.macos.v0.2.0.tgz
    download $filename
    cp fractal-bin/gftl /usr/local/bin/
    cp fractal-bin/gtool /usr/local/bin/
    cp fractal-bin/libwasmlib.dylib /usr/local/lib/
else
   OS_NAME=$( cat /etc/os-release | grep ^NAME | cut -d'=' -f2 | sed 's/\"//gI' )

   case "$OS_NAME" in
      "Amazon Linux")
         echo "installing fractal apps in Amazon Linux AMI"
         filename=fractal-bin.ami2.v0.2.0.tgz
         download $filename
         cp fractal-bin/gftl /usr/local/bin/
         cp fractal-bin/gtool /usr/local/bin/
         cp fractal-bin/libwasmlib.so /usr/lib64/
         ;;
      "CentOS Linux")
         echo "installing fractal apps in CentOS Linux"
         filename=fractal-bin.centos.v0.2.0.tgz
         download $filename
         cp fractal-bin/gftl /usr/local/bin/
         cp fractal-bin/gtool /usr/local/bin/
         cp fractal-bin/libwasmlib.so /usr/lib64/
         ;;
      "Ubuntu")
         echo "installing fractal apps in Ubuntu Linux"
         filename=fractal-bin.ubuntu.v0.2.0.tgz
         download $filename
         cp fractal-bin/gftl /usr/local/bin/
         cp fractal-bin/gtool /usr/local/bin/
         cp fractal-bin/libwasmlib.so /usr/lib/
         ;;
      *)
         printf "\\n\\tUnsupported Linux Distribution. Exiting now.\\n\\n"
         exit 1
   esac
fi

# check version
gftl --help | grep -A1 VERSION
if [ "$?" != "0" ];then
    printf "\\n\\tGet fractal bin version failed. Exiting now.\\n\\n"
    exit 1
fi

printf "\\n\\tInstall fractal success.\\n\\n"
exit 0
