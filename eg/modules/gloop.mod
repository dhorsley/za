
define setballcolour(x)
    return "[#{x}]o[#-]"
enddef

define setplayercolour(x)
    return "[#{x}]^^^^^^^[#-]"
enddef

define setlivesicons(x)
    hicons="♡ ♡ ♡ ♡ ♡        "
    return format("[#2]%-12s[#-]",substr(hicons,0,x*4))
enddef

define ball_lost()
    setglob moving=false
    setglob bx=getglob("px")+3
    setglob by=getglob("py")-1
    setglob bs=setballcolour(6)
    setglob start_ball_y=getglob("by")
    setglob box=0
    setglob boy=0
    l=getglob("lives")
    l=l-1
    setglob lives=l
    setglob hs=setlivesicons(l)
enddef

define make_layer(n,bricks,mw)
    row=bricks["{n}-row"]
    col=bricks["{n}-colour"]
    cen=int(mw)
    start=cen-(len(bricks["{n}"])/2)*8
    sp=getglob("sprites")
    foreach b in bricks["{n}"]
        if b
            k=key_b+1
            sx=start+key_b*8
            sy=row
            ss="[#"+bricks["{n}-colour"]+"]"+bricks["{n}-type"]+"[#-]"
            sp["brick-{row}-{k}"]=[0,sx,sy,ss,"        "]
        endif
    endfor
    setglob sprites = sp
enddef

define make_brick(n)
    left = "["
    right = "]"
    mid = "-"
    return " "+left+mid+mid+mid+mid+right+" "
enddef

define newbricks(n,w)
    for f=1 to w/8
        fulllayer=append(fulllayer,true)
    endfor
    bricks["{n}-colour"]=rand(7)
    bricks["{n}"]=fulllayer
    bricks["{n}-row"]=10+n
    bricks["{n}-type"]=make_brick(rand(4))
    return bricks
enddef

define make_all_layers(n,mw)
    for e = 1 to n
        bricks=newbricks(e,mw)
        make_layer(e,bricks,mw)
    endfor
enddef

