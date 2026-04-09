#!/bin/bash

# 创建 bin 目录
mkdir -p bin

# 下载 controller-gen 二进制文件
wget -O bin/controller-gen https://github.com/kubernetes-sigs/controller-tools/releases/download/v0.14.0/controller-gen_0.14.0_linux_amd64.tar.gz

# 解压文件
tar -xzf bin/controller-gen -C bin/

# 移除压缩文件
rm bin/controller-gen

# 重命名二进制文件
mv bin/controller-gen_0.14.0_linux_amd64 bin/controller-gen

# 添加执行权限
chmod +x bin/controller-gen

echo "Controller-gen installed successfully!"

# 验证安装
./bin/controller-gen --version