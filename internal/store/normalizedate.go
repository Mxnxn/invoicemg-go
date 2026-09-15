package store

import "regexp"

var (
	isoDate  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	toString = regexp.MustCompile(`^\w+ (\w+) (\d{1,2}) (\d{4})`)
	gstMonth = map[string]string{"Jan": "01", "Feb": "02", "Mar": "03", "Apr": "04", "May": "05", "Jun": "06", "Jul": "07", "Aug": "08", "Sep": "09", "Oct": "10", "Nov": "11", "Dec": "12"}
)

// NormalizeDate ports Helpers/NormalizeDate.js: YYYY-MM-DD passes through, a JS Date toString
// ("Wed Sep 10 2026 ...") converts, anything else returns unchanged.
func NormalizeDate(raw string) string {
	if raw == "" || isoDate.MatchString(raw) {
		return raw
	}
	if m := toString.FindStringSubmatch(raw); m != nil {
		mon := gstMonth[m[1]]
		if mon == "" {
			mon = "01"
		}
		day := m[2]
		if len(day) == 1 {
			day = "0" + day
		}
		return m[3] + "-" + mon + "-" + day
	}
	return raw
}
