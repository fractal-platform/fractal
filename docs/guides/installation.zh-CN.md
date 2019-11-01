[English](installation.md) | 简体中文

# 安装Fractal程序

## 先决条件
支持的操作系统：

    * macOS（版本：10.14.6或更高版本）
    * CentOS Linux（版本：7.6.1810或更高版本）
    * Ubuntu Linux（版本：18.04.2或更高版本）
    * Amazon Linux 2

最低硬件要求：

    * 2核CPU
    * 4GB内存
    * 100GB硬盘
    * 10Mbps网络带宽

## 快速安装
1.启动Terminal应用程序。
2.获取安装脚本，然后在Terminal中运行它：
```
    $ curl -O -L https://github.com/fractal-platform/fractal/releases/download/v0.2.1/install.sh
    $ bash install.sh
    VERSION:
        0.2.1-stable-1328975
        Install fractal success.
```
*您应该将版本号更改为最新的发行标签。*

如果您在Terminal中获得VERSION，则表示安装成功。

## 详细安装步骤
1. 获取安装包。访问我们的[github release page](https://github.com/fractal-platform/fractal/releases)，并下载相应平台和版本的tgz文件。
2. 解压缩tgz文件，您将获得二进制文件(gftl/gtool)和库文件。
3. 将二进制文件复制到系统bin路径（建议使用*/usr/local/bin/*）。
4. 将库文件复制到系统库路径（建议使用*/usr/lib64/*或*/usr/lib/*）。
5. 测试。启动Terminal应用程序，然后运行：
```
    $ gftl --help
    gftl [options]
    
    VERSION:
       0.2.1-stable-8bab622
    ...
    ...
```
如果您在Terminal中获得VERSION，则表示安装成功。