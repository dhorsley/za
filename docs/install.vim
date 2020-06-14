#!/bin/bash

# this is only going to work on ubuntu at best...
#  you'll probably need to do this manually until i have the
#  patience to write a generic script. feel free to contribute one!

pth=$(ls -d /usr/share/vim/vim8*)
echo "PATH : |$pth|"

sudo cp -f za.vim $pth/ftplugin/
sudo cp -f za.vim $pth/syntax/
sudo chmod 755 $pth/syntax/za.vim

