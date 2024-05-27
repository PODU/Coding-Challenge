// Package svghtml renders the calendar as SVG, or as an HTML page around the
// SVG. Output is built with plain string formatting so it is byte-for-byte
// stable for the golden tests.
package svghtml

import (
	"fmt"
	"strings"

	"gitgraph/internal/model"
)

const (
	cellSize = 11
	cellGap  = 3
	topPad   = 20 // room for month labels
	leftPad  = 30 // room for weekday labels
)

// GitHub's greens, levels 0 to 4.
var levelColor = [5]string{"#ebedf0", "#9be9a8", "#40c463", "#30a14e", "#216e39"}

var weekdayLabel = [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

func shortMonth(m int) string {
	names := [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	if m < 1 || m > 12 {
		return "???"
	}
	return names[m-1]
}

func SVG(cal model.Calendar) string {
	weeks := cal.Weeks
	width := leftPad + len(weeks)*(cellSize+cellGap)
	height := topPad + 7*(cellSize+cellGap)

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" font-family="Helvetica, Arial, sans-serif" font-size="9">`, width, height)
	b.WriteByte('\n')

	// Like GitHub, only label Mon, Wed and Fri.
	for row := 0; row < 7; row++ {
		wd := (int(cal.FirstDay) + row) % 7
		if wd == 1 || wd == 3 || wd == 5 {
			y := topPad + row*(cellSize+cellGap) + cellSize - 1
			fmt.Fprintf(&b, `  <text x="0" y="%d" fill="#767676">%s</text>`+"\n", y, weekdayLabel[wd])
		}
	}

	prevMonth := 0
	for wi, week := range weeks {
		d := week[0].Date
		if int(d.Month) != prevMonth {
			prevMonth = int(d.Month)
			x := leftPad + wi*(cellSize+cellGap)
			fmt.Fprintf(&b, `  <text x="%d" y="10" fill="#767676">%s</text>`+"\n", x, shortMonth(int(d.Month)))
		}
	}

	for wi, week := range weeks {
		for row := 0; row < 7; row++ {
			c := week[row]
			x := leftPad + wi*(cellSize+cellGap)
			y := topPad + row*(cellSize+cellGap)
			fmt.Fprintf(&b,
				`  <rect x="%d" y="%d" width="%d" height="%d" rx="2" ry="2" fill="%s"><title>%s: %d</title></rect>`+"\n",
				x, y, cellSize, cellSize, levelColor[c.Level], c.Date, c.Count)
		}
	}

	b.WriteString("</svg>\n")
	return b.String()
}

// HTML returns a page with the SVG and the stats.
func HTML(cal model.Calendar, stats model.Stats, title string) string {
	if title == "" {
		title = "git contributions"
	}
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", escapeHTML(title))
	b.WriteString("<style>body{font-family:Helvetica,Arial,sans-serif;margin:24px;color:#24292f}" +
		".stats{margin-top:12px;font-size:13px}.stats span{margin-right:16px}</style>\n")
	b.WriteString("</head>\n<body>\n")
	fmt.Fprintf(&b, "<h2>%s</h2>\n", escapeHTML(title))
	b.WriteString(SVG(cal))
	b.WriteString("<div class=\"stats\">\n")
	fmt.Fprintf(&b, "<span><b>%d</b> commits in %d days</span>", stats.Total, stats.Days)
	fmt.Fprintf(&b, "<span><b>%d</b> active days</span>", stats.ActiveDays)
	fmt.Fprintf(&b, "<span>current streak <b>%d</b></span>", stats.CurrentStreak)
	fmt.Fprintf(&b, "<span>longest streak <b>%d</b></span>\n", stats.LongestStreak)
	b.WriteString("</div>\n</body>\n</html>\n")
	return b.String()
}

func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
