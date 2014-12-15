package main

/*
 * Auswertung meiner Heatpump Datenbank 2014-12-12 mih
 *
 * TODO
 * Gesamtübersicht: alle Werte insgesamt -- wenn weder Jahr noch Monat angegeben sind
 * Jahresübersicht: alle Werte eines Jahres, Beispiel -y 2010
 * Monatsübersicht: Werte eines Monats, -y 2010 -m 2
 * Tag:		    Wert eines Tages, -y 2011 -m 10 -d 30
 * Monate/Jahr:     Nur die Summen der Monate eines Jahres, -y 2014 -sum
 * Monate/Jahre:    Die Monatssummen aller Jahre / inkl. Jahresausgabe -alls
 *
 * Alle Ausgaben auch in CVS Dateien
 */

import (
        "flag"
        "fmt"
        "os"
	"strconv"
        "code.google.com/p/go-sqlite/go1/sqlite3"
)

const HeatpumpDB = "/home/mhusmann/usr/src/gocode/src/github.com/mhusmann/heatresults/heatpump.db"
const HeaderLine =
	"Jahr/Monat;\tJan;\tFeb;\tMar;\tApr;\tMai;\tJun;\tJul;\t"+
	"Aug;\tSep;\tOct;\tNov;\tDec"

type Row struct {
        Id    int
        Date  string
        Ht    int
        Nt    int
        HtNt  int
        Tarif int
}

func monthlyValues(conn* sqlite3.Conn, year string, month int) (ht, nt, htnt int, average float64) {
	m := fmt.Sprintf("%02d", month)

	var lastDate string
	var noOfDays int
	stmt := fmt.Sprintf("SELECT COUNT(day), MAX(day) FROM dayly WHERE day LIKE %q",
			    year+"-"+m+"%")
	for s, err := conn.Query(stmt); err == nil; err = s.Next() {
                s.Scan(&noOfDays, &lastDate)
	}

	if lastDate == "2009-04-30" {
		stmt = "select max(ht), max(nt), max(ht)+max(nt) "+
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

	for s, err := conn.Query(stmt); err == nil; err = s.Next() {
                s.Scan(&ht, &nt, &htnt)
	}

	average = float64(htnt)/float64(noOfDays)
	return ht, nt, htnt, average
}

func sumYear(conn* sqlite3.Conn, year string) {
	// request min and max month of the year from database
	stmt := fmt.Sprintf("SELECT strftime(%q, MIN(day)), strftime(%q, MAX(day)) "+
		"FROM dayly WHERE day LIKE %q", "%m", "%m", year+"%")

	var startMonth, endMonth string
	for s, err := conn.Query(stmt); err == nil; err = s.Next() {
                s.Scan(&startMonth, &endMonth)
	}

	fmt.Println("# Jahresübersicht: ", year)
	fmt.Println("# Monat  HT     NT  HT+NT AVG(HT+NT)")

	var ht, nt, htnt, sumHt, sumNt, sumHtNt int
	var avg float64
	endm, _ := strconv.Atoi(endMonth)
	for i,_ := strconv.Atoi(startMonth); i <= endm; i++ {
		ht, nt, htnt, avg = monthlyValues(conn, year, i)
		fmt.Printf("%4d;%5d;%5d;%5d;%6.2f\n", i, ht, nt, htnt, avg)
		sumHt += ht
		sumNt += nt
		sumHtNt += htnt
	}
	fmt.Printf("# Totals: Ht %3d, Nt %3d, HtNt %3d\n", sumHt, sumNt, sumHtNt)
}

func theRest(conn* sqlite3.Conn, requestedDate string) {
	fmt.Println("# requestedDate", requestedDate)

	stmt := fmt.Sprintf("select d.id, d.day 'day', d.ht-p.ht HT, d.nt-p.nt NT, "+
	"d.ht-p.ht + d.nt-p.nt 'HT+NT' "+
	"from dayly d "+
	"join dayly p "+
	"on date(d.day,'-1 day') = p.day "+
	"where d.day like %q", requestedDate)

        sumHt, sumNt, sumHtNt, countDays := 0, 0, 0, 0
        var aRow Row

        for s, err := conn.Query(stmt); err == nil; err = s.Next() {
                s.Scan(&aRow.Id, &aRow.Date, &aRow.Ht, &aRow.Nt, &aRow.HtNt)
                fmt.Printf("%6d\t%s\t%3d\t%3d\t%3d\n", aRow.Id, aRow.Date, aRow.Ht, aRow.Nt, aRow.HtNt)
                sumHt += aRow.Ht
                sumNt += aRow.Nt
                sumHtNt += aRow.HtNt
                countDays += 1
        }
        fmt.Printf("# Totals: HT: %d, NT: %d, HT+NT: %d, days: %d, avg/day: %3.2f kWh\n",
                sumHt, sumNt, sumHtNt, countDays, float64(sumHtNt)/float64(countDays))
}

func total(conn* sqlite3.Conn, month, year int) (int) {
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
	for s, err := conn.Query(stmt); err == nil; err = s.Next() {
                s.Scan(&htnt)
	}
	return htnt
}

func allSums(conn* sqlite3.Conn) {
	stmt := fmt.Sprintf(`select strftime(%q, min(day)), strftime(%q, min(day)), `+
		`strftime(%q, max(day)), strftime(%q, max(day)) `+
		`from dayly`, "%m","%Y", "%m", "%Y")
	var startMonth, startYear, endMonth, endYear string
	for s, err := conn.Query(stmt); err == nil; err = s.Next() {
                s.Scan(&startMonth, &startYear, &endMonth, &endYear)
	}
	startM, _ := strconv.Atoi(startMonth)
	startY, _ := strconv.Atoi(startYear)
	endM, _ := strconv.Atoi(endMonth)
	endY, _ := strconv.Atoi(endYear)
// 	fmt.Println(startMonth, startM, startYear, startY, endMonth, endM, endYear, endY)
	fmt.Println(HeaderLine)
	var em int
	for sy := startY; sy <= endY; sy++ {
		if sy < endY {
			em = 12
		} else {
			em = endM
		}
		fmt.Printf("%8d;", sy)
		for i := 1; i < startM; i++ {
			fmt.Printf("\t;")
		}
		for sm := startM; sm <= em; sm++ {
			fmt.Printf("\t%3d;",total(conn, sm, sy))
		}
		startM = 1
		fmt.Println()
	}
}

func main() {
        var db, y, m, d string
        var sum, alls bool
        flag.StringVar(&db, "db", HeatpumpDB, "Pfad/Name der Datenbank")
        flag.StringVar(&y, "y", "", "zu berechnendes Jahr")
        flag.StringVar(&m, "m", "", "Monat")
        flag.StringVar(&d, "d", "", "Tag")
	flag.BoolVar(&sum, "sum", false, "Nur Summe")
	flag.BoolVar(&alls, "alls", false, "Summen aller Jahre als Übersicht")
        flag.Parse()

        fmt.Println("# Datenbank:", db)
        fmt.Println("# Jahr:", y)
        fmt.Println("# Monat:", m)
        fmt.Println("# Tag:", d)
	fmt.Println("# Summen:", sum)
	fmt.Println("# Alle Summen:", alls)
        fmt.Println("# tail:", flag.Args())

        conn, err := sqlite3.Open(HeatpumpDB)
        if err != nil {
                fmt.Println("# kann Datenbank %s nicht öffnen: %s", HeatpumpDB, err)
                os.Exit(1)
        }
        defer conn.Close()

        switch {
		default: theRest(conn, y + "%")
		case alls == true: allSums(conn)
		case sum == true: sumYear(conn, y)
		case m != "" && d != "": theRest(conn, y + "-" + m + "-" + d)
		case m != "": theRest(conn, y + "-" + m + "%")
	}
}
