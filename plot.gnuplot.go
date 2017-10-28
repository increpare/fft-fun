
set term png   
set output "printme.png"

set view map
set dgrid3d
splot "/tmp/test.gnuplot" using 1:2:3 with pm3d