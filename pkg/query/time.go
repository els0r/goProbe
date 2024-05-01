/////////////////////////////////////////////////////////////////////////////////
//
// time.go
//
// Wrapper for time parsing functions
//
// Written by Lennart Elsen lel@open.ch, April 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package query

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/els0r/goProbe/pkg/types"
)

// TimeFormat denotes a time format with an optional verbose name for display
type TimeFormat struct {
	Name   string
	Format string
}

var (
	timeFormatsDefault = []TimeFormat{
		{"RFC3339", time.RFC3339},                    // "2006-01-02T15:04:05Z07:00"
		{"ANSIC", time.ANSIC},                        // "Mon Jan _2 15:04:05 2006"
		{"RUBY DATE", time.RubyDate},                 // "Mon Jan 02 15:04:05 -0700 2006"
		{"RFC822 with numeric zone", time.RFC822Z},   // "02 Jan 06 15:04 -0700" // RFC822 with numeric zone
		{"RFC1123 with numeric zone", time.RFC1123Z}, // "Mon, 02 Jan 2006 15:04:05 -0700" // RFC1123 with numeric zone
	}

	timeFormatsCustom = []TimeFormat{
		{"CUSTOM", types.DefaultTimeOutputFormat},

		// unnamed additions
		{"", "2006-01-02 15:04:05 -0700"},
		{"", "2006-01-02 15:04 -0700"},
		{"", "2006-01-02 15:04:05"},
		{"", "2006-01-02 15:04"},
		{"", "06-01-02 15:04:05 -0700"},
		{"", "06-01-02 15:04 -0700"},
		{"", "06-01-02 15:04:05"},
		{"", "06-01-02 15:04"},
		{"", "02-01-2006 15:04:05 -0700"},
		{"", "02-01-2006 15:04 -0700"},
		{"", "02-01-2006 15:04:05"},
		{"", "02-01-2006 15:04"},
		{"", "02-01-06 15:04:05 -0700"},
		{"", "02-01-06 15:04 -0700"},
		{"", "02-01-06 15:04:05"},
		{"", "02-01-06 15:04"},
		{"", "02.01.2006 15:04"},
		{"", "02.01.2006 15:04 -0700"},
		{"", "02.01.06 15:04"},
		{"", "02.01.06 15:04 -0700"},
		{"", "2.1.06 15:04:05"},
		{"", "2.1.06 15:04:05 -0700"},
		{"", "2.1.06 15:04"},
		{"", "2.1.06 15:04 -0700"},
		{"", "2.1.2006 15:04:05"},
		{"", "2.1.2006 15:04:05 -0700"},
		{"", "2.1.2006 15:04"},
		{"", "2.1.2006 15:04 -0700"},
		{"", "02.1.2006 15:04:05"},
		{"", "02.1.2006 15:04:05 -0700"},
		{"", "02.1.2006 15:04"},
		{"", "02.1.2006 15:04 -0700"},
		{"", "2.01.2006 15:04:05"},
		{"", "2.01.2006 15:04:05 -0700"},
		{"", "2.01.2006 15:04"},
		{"", "2.01.2006 15:04 -0700"},
		{"", "02.1.06 15:04:05"},
		{"", "02.1.06 15:04:05 -0700"},
		{"", "02.1.06 15:04"},
		{"", "02.1.06 15:04 -0700"},
		{"", "2.01.06 15:04:05"},
		{"", "2.01.06 15:04:05 -0700"},
		{"", "2.01.06 15:04"},
		{"", "2.01.06 15:04 -0700"},
	}

	timeFormatsRelative = []TimeFormat{
		{"RELATIVE", "-15d:04h:05m"},
		{"", "-15d4h5m"},
	}
)

// TimeFormatsDefault returns a list of all supported default time formats
func TimeFormatsDefault() []TimeFormat {
	return timeFormatsDefault
}

// TimeFormatsCustom returns a list of all supported custom time formats
func TimeFormatsCustom() []TimeFormat {
	return timeFormatsCustom
}

// TimeFormatsRelative returns a list of all supported relative time formats
func TimeFormatsRelative() []TimeFormat {
	return timeFormatsRelative
}

// function returning a UNIX timestamp relative to the current time
func parseRelativeTime(rtime string) (int64, error) {
	if len(rtime) == 0 {
		return 0, fmt.Errorf("empty relative time")
	}
	if rtime[0] != '-' {
		return 0, fmt.Errorf("expecting leading '-' for relative time")
	}

	rtime = rtime[1:]

	var secBackwards int64

	// support for time.Duration string
	if !strings.Contains(rtime, ":") {
		var ds string
		if strings.Contains(rtime, "d") {
			s := strings.Split(rtime, "d")
			if s[0] == "" {
				return 0, fmt.Errorf("expecting number before 'd' token")
			}

			num, err := strconv.ParseInt(s[0], 10, 64)
			if err != nil {
				return 0, err
			}
			secBackwards += 86400 * num
			ds = strings.Join(s[1:], "")

			// return if only a "d" duration was supplied
			if ds == "" {
				return (time.Now().Unix() - secBackwards), nil
			}
		} else {
			ds = rtime
		}

		d, err := time.ParseDuration(ds)
		if err != nil {
			return 0, fmt.Errorf("failed to parse %q as duration: %w", rtime, err)
		}
		secBackwards += int64(d.Seconds())
	} else {
		// iterate over different time chunks to get the days, hours and minutes
		for _, chunk := range strings.Split(rtime, ":") {
			var err error

			if len(chunk) == 0 {
				return 0, fmt.Errorf("incorrect relative time specification %q", rtime)
			}

			num := int64(0)

			switch chunk[len(chunk)-1] {
			case 'd':
				if num, err = strconv.ParseInt(chunk[:len(chunk)-1], 10, 64); err != nil {
					return 0, err
				}
				secBackwards += 86400 * num
			case 'h':
				if num, err = strconv.ParseInt(chunk[:len(chunk)-1], 10, 64); err != nil {
					return 0, err
				}
				secBackwards += 3600 * num
			case 'm':
				if num, err = strconv.ParseInt(chunk[:len(chunk)-1], 10, 64); err != nil {
					return 0, err
				}
				secBackwards += 60 * num
			case 's':
				if num, err = strconv.ParseInt(chunk[:len(chunk)-1], 10, 64); err != nil {
					return 0, err
				}
				secBackwards += num
			default:
				return 0, errors.New("incorrect relative time specification")
			}
		}
	}
	return (time.Now().Unix() - secBackwards), nil
}

var (
	errorInvalidTimeFormat   = errors.New("invalid time format")
	errorInvalidTimeInterval = errors.New("invalid time interval")
)

// ParseTimeRange will run ParseTimeArgument for a range and validate if the interval is
// non-zero
func ParseTimeRange(firstStr, lastStr string) (first, last int64, err error) {
	if firstStr != "" {
		first, err = ParseTimeArgument(firstStr)
		if err != nil {
			err = fmt.Errorf("%w for --first: %w", errorInvalidTimeFormat, err)
			return
		}
	}

	if lastStr == "" {
		last = time.Now().Unix()
	} else {
		last, err = ParseTimeArgument(lastStr)
		if err != nil {
			err = fmt.Errorf("%w for --last: %w", errorInvalidTimeFormat, err)
			return
		}
	}

	if first > last {
		err = fmt.Errorf("%w: the lower time bound cannot be greater than the upper time bound", errorInvalidTimeInterval)
		return
	}
	return first, last, nil
}

// ParseTimeRangeCollectErrors will run ParseTimeArgument for a range and validate if the interval is
// non-zero. It will append errors encountered during interval validation to the huma.ErrorDetail slice and
// return them. The error condition will thus be len(details) > 0
func ParseTimeRangeCollectErrors(firstStr, lastStr string) (first, last int64, details []*huma.ErrorDetail) {
	var err error
	if firstStr != "" {
		first, err = ParseTimeArgument(firstStr)
		if err != nil {
			details = append(details, &huma.ErrorDetail{
				Location: "first",
				Message:  fmt.Sprintf("%s: %s", errorInvalidTimeFormat, err),
				Value:    firstStr,
			})
		}
	}

	if lastStr == "" {
		last = time.Now().Unix()
	} else {
		last, err = ParseTimeArgument(lastStr)
		if err != nil {
			details = append(details, &huma.ErrorDetail{
				Location: "last",
				Message:  fmt.Sprintf("%s: %s", errorInvalidTimeFormat, err),
				Value:    lastStr,
			})
		}
	}

	if first > last {
		details = append(details, &huma.ErrorDetail{
			Location: "last",
			Message:  fmt.Sprintf("%s: the lower time bound cannot be greater than the upper time bound", errorInvalidTimeInterval),
			Value:    firstStr,
		})
	}
	return first, last, details
}

// ParseTimeArgument is the entry point for external calls and converts valid formats to a unix timtestamp
func ParseTimeArgument(timeString string) (int64, error) {
	var (
		t    time.Time
		tRel int64
	)

	// incorporate location information
	loc, err := time.LoadLocation("Local")
	if err != nil {
		return int64(0), err
	}

	// check whether a relative timestamp was specified
	if timeString[0] == '-' {
		tRel, err = parseRelativeTime(timeString)
		if err != nil {
			return 0, err
		}
		return tRel, err
	}

	// try to interpret string as unix timestamp
	i, err := strconv.ParseInt(timeString, 10, 64)
	if err == nil {
		return i, nil
	}

	// then check other time formats
	for _, tFormat := range append(timeFormatsDefault, timeFormatsCustom...) {
		t, err = time.ParseInLocation(tFormat.Format, timeString, loc)
		if err == nil {
			return t.Unix(), nil
		}
	}

	return 0, fmt.Errorf("unable to parse time format: %w", err)
}
