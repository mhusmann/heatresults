##set terminal pbm color
##set output "statistic.pbm"
set title "Dezember 2014"
#set xrange [1:31]
set xlabel "Datum"; set ylabel "kWh"
set xtics rotate by -30
#set key top left
set key autotitle columnhead
set boxwidth 0.6
set style fill solid 0.4 border
set grid
#set xtics ("Jan" 1, "Feb" 2, "Mar" 3, "Apr" 4, "May" 5,\
#            "Jun" 6, "Jul" 7, "Aug" 8, "Sep" 9, "Oct" 10,\
#            "Nov" 11, "Dec" 12)

plot '2014-12.dat' using 0:3:xtic(2) t " HT " with boxes lt 7,\
    '' u ($0):($4) t " NT " with linesp lt 3,\
    '' u ($0):($5) t " HT+NT " with linespoints lt 4
pwd
pause -1 "Press a key"
