package main

/*
 * Auswertung meiner Heatpump Datenbank 2014-12-12 mih
 * letzte Änderung Fr 23. Jan 11:58:03 CET 2015
 *
 * Gesamtübersicht: alle Werte insgesamt -- wenn weder Jahr noch Monat angegeben sind
 * Jahresübersicht: alle Werte eines Jahres, Beispiel -y 2010
 * Monatsübersicht: Werte eines Monats, -y 2010 -m 2
 * Tag:		    Wert eines Tages, -y 2011 -m 10 -d 30
 * Monate/Jahr:     Nur die Summen der Monate eines Jahres, -y 2014 -sum
 * Monate/Jahre:    Die Monatssummen aller Jahre / inkl. Jahresausgabe -alls
 *
 * Alle Ausgaben auch in CSV Dateien
 */

import (
	"code.google.com/p/go-sqlite/go1/sqlite3"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const pltSt1 = `
## set terminal pbm color
##set output "statistic.pbm"
set term qt persist
set title "Verbrauchswerte"
## set xrange [1:31]
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
`

const pltSt2 = `plot '%s' using 2:xtic(1) with linesp lt 1,\`

const heatpumpDB = "/home/mhusmann/Documents/src/pyt/heizung/heatpump.db"
const headerLine = "Jahr/Monat\tJan\tFeb\tMar\tApr\tMai\tJun\tJul\t" +
	"Aug\tSep\tOct\tNov\tDec"

var theMonths = [12]string{"Jan", "Feb", "Mar", "Apr", "Mai",
	"Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

type row struct {
	id    int
	date  string
	ht    int
	nt    int
	htnt  int
	tarif int
}

type plot struct {
	datName string
	pltName string
	datFile *os.File
	pltFile *os.File
}

func (p *plot) init(requestedDate string, pltRest int, years ...int) {
	var err error
	fmt.Println(years)

	p.pltName = strings.Replace(requestedDate, "%", "", -1) + ".plt"
	p.pltFile, err = os.Create(p.pltName)
	if err != nil {
		panic(err)
	}

	p.datName = strings.Replace(requestedDate, "%", "", -1) + ".dat"

	p.datFile, err = os.Create(p.datName)
	if err != nil {
		panic(err)
	}

	if pltRest < 2 {
		_, err := io.WriteString(p.datFile, "the first line\n")
		if err != nil {
			panic(err)
		}
	}

	_, err = io.WriteString(p.pltFile, pltSt1)
	if err != nil {
		panic(err)
	}

	var pltSt string
	switch pltRest {
	case 0:
		pltSt = fmt.Sprintf(` plot '%s' using 3:xtic(2) t " HT " with boxes lt 7,\
    			'' u 4 t " NT " with linesp lt 3,\
    			'' u 5 t " HT+NT " with linespoints lt 4`, p.datName)

	case 1:
		pltSt = fmt.Sprintf(` plot '%s' using 2:xtic(1) t " HT " with boxes lt 7,\
    			'' u 3 t " NT " with linesp lt 3,\
    			'' u 4 t " HT+NT " with linespoints lt 4`, p.datName)

	case 2:
		// write the appropriate plt file which must be extended by
		// every additional year
		pltSt = fmt.Sprintf(pltSt2, p.datName)
		for i := 2; i <= years[1]-years[0]+1; i++ {
			pltSt += fmt.Sprintf("\n'' u %d w linesp lt %d,\\", i+1, i)
		}
	}

	_, err = io.WriteString(p.pltFile, pltSt)
	if err != nil {
		panic(err)
	}
}

func (p *plot) close() {
	fmt.Println("# closing plot files")
	p.datFile.Close()
	p.pltFile.Close()
}

func (p *plot) writeDat(st string) {
	_, err := io.WriteString(p.datFile, st)
	if err != nil {
		panic(err)
	}
}

func (p *plot) gnuplt(pltName string) {
	_, err := exec.Command("gnuplot", p.pltName).Output()
	if err != nil {
		panic(err)
	}
}

func monthlyValues(conn *sqlite3.Conn, year string, month int) (ht, nt, htnt int, average float64) {
	m := fmt.Sprintf("%02d", month)

	var lastDate string
	var noOfDays int
	stmt := fmt.Sprintf("SELECT COUNT(day), MAX(day) FROM dayly WHERE day LIKE %q",
		year+"-"+m+"%")
	for dbResult, err := conn.Query(stmt); err == nil; err = dbResult.Next() {
		dbResult.Scan(&noOfDays, &lastDate)
	}

	if lastDate == "2009-04-30" {
		stmt = "select max(ht), max(nt), max(ht)+max(nt) " +
			"from dayly where day like '2009-04%'"
	} else {
		stmt = fmt.Sprintf("SELECT SUM(d.ht-p.ht) HT, "+
			"SUM(d.nt-p.nt) NT, "+
			"SUM(d.ht-p.ht + d.nt-p.nt) 'HT+NT' "+
			"FROM dayly d "+
			"JOIN dayly p "+
			"ON DATE(d.day, 'start of month','-1 day') = p.day "+
			"WHERE d.day = %q", lastDate)
	}

	for dbResult, err := conn.Query(stmt); err == nil; err = dbResult.Next() {
		dbResult.Scan(&ht, &nt, &htnt)
	}

	average = float64(htnt) / float64(noOfDays)
	return ht, nt, htnt, average
}

func sumYear(conn *sqlite3.Conn, g bool, year string) {
	// request min and max month of the year from database
	p := new(plot)
	if g {
		p.init(year, 1)
		defer p.close()
	}

	stmt := fmt.Sprintf("SELECT strftime(%q, MIN(day)), strftime(%q, MAX(day)) "+
		"FROM dayly WHERE day LIKE %q", "%m", "%m", year+"%")

	var startMonth, endMonth string
	for dbResult, err := conn.Query(stmt); err == nil; err = dbResult.Next() {
		dbResult.Scan(&startMonth, &endMonth)
	}

	fmt.Println("# Jahresübersicht: ", year)
	fmt.Println("# Monat  HT     NT  HT+NT AVG(HT+NT)")

	var ht, nt, htnt, sumHt, sumNt, sumHtNt int
	var avg float64
	endm, _ := strconv.Atoi(endMonth)
	for i, _ := strconv.Atoi(startMonth); i <= endm; i++ {
		ht, nt, htnt, avg = monthlyValues(conn, year, i)
		outSt := fmt.Sprintf("%4s %5d %5d %5d %6.2f\n", theMonths[i-1], ht, nt, htnt, avg)
		fmt.Printf(outSt)
		if g {
			p.writeDat(outSt)
		}
		sumHt += ht
		sumNt += nt
		sumHtNt += htnt
	}
	fmt.Printf("# Totals: Ht %3d, Nt %3d, HtNt %3d\n", sumHt, sumNt, sumHtNt)

	if g {
		p.gnuplt(p.pltName)
	}
}

func theRest(conn *sqlite3.Conn, g bool, requestedDate string) {
	fmt.Println("# requestedDate", requestedDate)
	p := new(plot)
	if g {
		p.init(requestedDate, 0)
		defer p.close()
	}

	sumHt, sumNt, sumHtNt, countDays := 0, 0, 0, 0
	var aRow row

	stmt := fmt.Sprintf("select d.id, d.day 'day', d.ht-p.ht HT, d.nt-p.nt NT, "+
		"d.ht-p.ht + d.nt-p.nt 'HT+NT' "+
		"from dayly d "+
		"join dayly p "+
		"on date(d.day,'-1 day') = p.day "+
		"where d.day like %q", requestedDate)

	for dbResult, err := conn.Query(stmt); err == nil; err = dbResult.Next() {
		dbResult.Scan(&aRow.id, &aRow.date, &aRow.ht, &aRow.nt, &aRow.htnt)
		outSt := fmt.Sprintf("%6d\t%s\t%3d\t%3d\t%3d\n", aRow.id, aRow.date, aRow.ht, aRow.nt, aRow.htnt)
		fmt.Printf(outSt)
		if g {
			p.writeDat(outSt)
		}
		sumHt += aRow.ht
		sumNt += aRow.nt
		sumHtNt += aRow.htnt
		countDays++
	}
	fmt.Printf("# Totals: HT: %d, NT: %d, HT+NT: %d, days: %d, avg/day: %3.2f kWh\n",
		sumHt, sumNt, sumHtNt, countDays, float64(sumHtNt)/float64(countDays))

	if g {
		p.gnuplt(p.pltName)
	}
}

func total(conn *sqlite3.Conn, month, year int) int {
	var stmt, datestr string
	if year == 2009 && month == 4 {
		stmt = fmt.Sprintf(`select max(ht) + max(nt) total `+
			`from dayly `+
			`where day like %q`, "2009-04%")
	} else {
		datestr = fmt.Sprintf(`%d-%02d`, year, month)
		stmt = fmt.Sprintf(`select (max(d.ht)-max(p.ht)) + (max(d.nt)-max(p.nt)) total `+
			`from dayly d `+
			`join dayly p `+
			`ON DATE(d.day, 'start of month','-1 day') = p.day `+
			`where d.day like %q`, datestr+"%")
	}

	var htnt int
	for dbResult, err := conn.Query(stmt); err == nil; err = dbResult.Next() {
		dbResult.Scan(&htnt)
	}
	return htnt
}

func allSums(conn *sqlite3.Conn, g bool) {
	// storing the results in a map. Year as key the months values as a slice
	var data = make(map[string][]string)

	stmt := fmt.Sprintf(`select strftime(%q, min(day)), `+
		`strftime(%q, min(day)), `+
		`strftime(%q, max(day)), strftime(%q, max(day)) `+
		`from dayly`, "%m", "%Y", "%m", "%Y")
	var startMonth, startYear, endMonth, endYear string
	for dbResult, err := conn.Query(stmt); err == nil; err = dbResult.Next() {
		dbResult.Scan(&startMonth, &startYear, &endMonth, &endYear)
	}

	startM := 1
	startY, _ := strconv.Atoi(startYear)
	endY, _ := strconv.Atoi(endYear)

	p := new(plot)
	if g {
		p.init("all-sums", 2, startY, endY)
		defer p.close()
	}

	fmt.Println(headerLine)
	em := 12
	for sy := startY; sy <= endY; sy++ {
		sySt := fmt.Sprint(sy)
		fmt.Printf("%8d", sy)
		for i := 1; i < startM; i++ {
			fmt.Printf("\t")
		}
		for sm := startM; sm <= em; sm++ {
			totals := total(conn, sm, sy)
			data[sySt] = append(data[sySt], fmt.Sprint(totals))
			fmt.Printf("\t%3d", totals)
		}
		startM = 1
		fmt.Println()
	}

	if g {
		p.writeDat(fmt.Sprintf("%d\t", 0))
		for year := startY; year <= endY; year++ {
			p.writeDat(fmt.Sprintf("%d\t", year))
		}
		p.writeDat("\n")
		for i := 1; i <= 12; i++ {
			p.writeDat(fmt.Sprintf("%s\t", theMonths[i-1]))
			for year := startY; year <= endY; year++ {
				p.writeDat(fmt.Sprintf("%s\t", data[fmt.Sprint(year)][i-1]))
			}
			p.writeDat("\n")
		}
		p.gnuplt(p.pltName)
	}
}

func main() {
	var db, y, m, d string
	var sum, alls, g bool
	flag.StringVar(&db, "db", heatpumpDB, "Pfad/Name der Datenbank")
	flag.StringVar(&y, "y", "", "zu berechnendes Jahr")
	flag.StringVar(&m, "m", "", "Monat")
	flag.StringVar(&d, "d", "", "Tag")
	flag.BoolVar(&sum, "sum", false, "Nur Summe")
	flag.BoolVar(&alls, "alls", false, "Summen aller Jahre als Übersicht")
	flag.BoolVar(&g, "g", false, "HTML Datei für graphische Übersicht")
	flag.Parse()

	fmt.Println("# Datenbank:", db)
	fmt.Println("# Jahr:", y)
	fmt.Println("# Monat:", m)
	fmt.Println("# Tag:", d)
	fmt.Println("# Summen:", sum)
	fmt.Println("# Alle Summen:", alls)
	fmt.Println("# tail:", flag.Args())

	conn, err := sqlite3.Open(heatpumpDB)
	if err != nil {
		fmt.Printf("# kann Datenbank %s nicht öffnen: %s\n",
			heatpumpDB, err)
		os.Exit(1)
	}
	defer conn.Close()

	switch {
	default:
		theRest(conn, g, y+"%")
	case alls == true:
		allSums(conn, g)
	case sum == true:
		sumYear(conn, g, y)
	case m != "" && d != "":
		theRest(conn, g, y+"-"+m+"-"+d)
	case m != "":
		theRest(conn, g, y+"-"+m+"%")
	}
}
