#!/usr/bin/perl
use warnings;
use strict;

my $t=0;
for(my $i = 0; $i <= 40000000; $i++){
    $t+=$i
}
print("total : $t\n");
