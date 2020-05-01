#!/bin/bash

pth=$(ls -d /usr/share/vim/vim8*)
echo "PATH : |$pth|"

sudo cp -f za.vim $pth/ftplugin/
sudo cp -f za.vim $pth/syntax/
sudo chmod 755 $pth/syntax/za.vim

