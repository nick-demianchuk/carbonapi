package date

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/bookingcom/carbonapi/pkg/parser"
)

var errBadTime = errors.New("bad time")
var timeNow = time.Now

// parseTime parses a time and returns hours and minutes
func parseTime(s string) (hour, minute int, err error) {

	switch s {
	case "midnight":
		return 0, 0, nil
	case "noon":
		return 12, 0, nil
	case "teatime":
		return 16, 0, nil
	}

	parts := strings.Split(s, ":")

	if len(parts) != 2 {
		return 0, 0, errBadTime
	}

	hour, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, errBadTime
	}

	minute, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, errBadTime
	}

	return hour, minute, nil
}

var TimeFormats = []string{"20060102", "01/02/06"}

// DateParamToEpoch turns a passed string parameter into a unix epoch
func DateParamToEpoch(s string, qtz string, d int64, defaultTimeZone *time.Location) int32 {

	if s == "" {
		// return the default if nothing was passed
		return int32(d)
	}

	// relative timestamp
	if s[0] == '-' {
		offset, err := parser.IntervalString(s, -1)
		if err != nil {
			return int32(d)
		}

		return int32(timeNow().Add(time.Duration(offset) * time.Second).Unix())
	}

	switch s {
	case "now":
		return int32(timeNow().Unix())
	case "midnight", "noon", "teatime":
		yy, mm, dd := timeNow().Date()
		hh, min, _ := parseTime(s) // error ignored, we know it's valid
		dt := time.Date(yy, mm, dd, hh, min, 0, 0, defaultTimeZone)
		return int32(dt.Unix())
	}

	sint, err := strconv.ParseInt(s, 10, 64)
	// need to check that len(s) > 8 to avoid turning 20060102 into seconds
	if err == nil && len(s) > 8 {
		return int32(sint) // We got a timestamp so returning it
	}

	s = strings.Replace(s, "_", " ", 1) // Go can't parse _ in date strings

	var ts, ds string
	split := strings.Fields(s)

	switch {
	case len(split) == 1:
		ds = s
	case len(split) == 2:
		ts, ds = split[0], split[1]
	case len(split) > 2:
		return int32(d)
	}

	var tz = defaultTimeZone
	if qtz != "" {
		if z, loadErr := time.LoadLocation(qtz); loadErr != nil {
			tz = z
		}
	}

	var t time.Time
dateStringSwitch:
	switch ds {
	case "today":
		t = timeNow()
		// nothing
	case "yesterday":
		t = timeNow().AddDate(0, 0, -1)
	case "tomorrow":
		t = timeNow().AddDate(0, 0, 1)
	default:
		for _, format := range TimeFormats {
			t, err = time.ParseInLocation(format, ds, tz)
			if err == nil {
				break dateStringSwitch
			}
		}

		return int32(d)
	}

	var hour, minute int
	if ts != "" {
		hour, minute, _ = parseTime(ts)
		// defaults to hour=0, minute=0 on error, which is midnight, which is fine for now
	}

	yy, mm, dd := t.Date()
	t = time.Date(yy, mm, dd, hour, minute, 0, 0, defaultTimeZone)

	return int32(t.Unix())
}
