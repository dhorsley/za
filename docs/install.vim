#!/usr/bin/za

#  you'll probably need to do this manually until i have the
#  patience to write a generic script. feel free to contribute one!

#
# first time:
#  you will need to add a stanza such as the one below to scripts.vim:
doc `
" Za shell scripts
  elseif s:name =~# '^za\>'
    set ft=za
`

if start(os(),"freebsd")
    pth=| ls -d /usr/local/share/vim/vim8*
else
    pth=| ls -d /usr/share/vim/vim8*
endif

println "PATH : |{pth}|"

| sudo cp -f za.vim {pth}/ftplugin/
| sudo cp -f za.vim {pth}/syntax/
| sudo chmod 755 {pth}/syntax/za.vim


