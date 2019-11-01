English | [简体中文](installation.zh-CN.md)

# Install Fractal Applications

## Prerequisite
Supported Operation Systems:

    * macOS(version: 10.14.6 or later)
    * CentOS Linux(version: 7.6.1810 or later)
    * Ubuntu Linux(version: 18.04.2 or later)
    * Amazon Linux 2

Hardware Minimum Requirements:

    * 2 vCPUs/cores
    * 4GB RAM
    * 100GB Disk
    * 10Mbps Network bandwidth

## Quick Installation
1. Start the terminal application.
2. Fetch install script, and run it in terminal:
```
    $ curl -O -L https://github.com/fractal-platform/fractal/releases/download/v0.2.1/install.sh
    $ bash install.sh
    VERSION:
        0.2.1-stable-1328975
        Install fractal success.
```
*You should change the version number to the latest release tag.*

If you get VERSION in terminal, it means that your installation is successful.

## Detailed Installation Steps
1. Fetch install package. Visit our [github release page](https://github.com/fractal-platform/fractal/releases), and download tgz file of corresponding platform and version.
2. Unpack tgz file, you will get binary files(gftl/gtool) and library files.
3. Copy binary files to system bin path(*/usr/local/bin/* is recommended).
4. Copy library files to system library path(*/usr/lib64/* or */usr/lib/* is recommended).
5. Test. Start the terminal application, and run:
```
    $ gftl --help
    gftl [options]
    
    VERSION:
       0.2.1-stable-8bab622
    ...
    ...
```
If you get VERSION in terminal, it means that your installation is successful.
