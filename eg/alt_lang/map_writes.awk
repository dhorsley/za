#!/usr/local/bin/gawk -f
BEGIN {
    n=20000000

    for (i=0; i<ARGC; i++) printf "argv[%d]->%s\n",i,ARGV[i]
    printf "argc->%d\n",ARGC
    printf "   n->%d\n",n

    for (i=0; i<n; i++) {
        d[i%1000]=i
    }
}

END {
   # print here if needed

   # for (i=0; i<length(d); i++) {
   #     printf "%d -> %d\n",i,d[i]
   # }

}

