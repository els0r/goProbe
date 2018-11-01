/////////////////////////////////////////////////////////////////////////////////
//
// DBTime.go
//
// Wrapper for time parsing functions
//
// Written by Lennart Elsen lel@open.ch, April 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import (
    "errors"
    "strconv"
    "strings"
    "time"
)

// Utility variables and functions for time parsing -----------------------------
var TimeFormats []string = []string{
    time.ANSIC,    // "Mon Jan _2 15:04:05 2006"
    time.RubyDate, // "Mon Jan 02 15:04:05 -0700 2006"
    time.RFC822Z,  // "02 Jan 06 15:04 -0700" // RFC822 with numeric zone
    time.RFC1123Z, // "Mon, 02 Jan 2006 15:04:05 -0700" // RFC1123 with numeric zone
    time.RFC3339,  // "2006-01-02T15:04:05Z07:00"

    // custom additions for MC
    "2006-01-02 15:04:05 -0700",
    "2006-01-02 15:04:05",
    "2006-01-02 15:04 -0700",
    "2006-01-02 15:04",
    "02.01.2006 15:04",
    "02.01.2006 15:04 -0700",
    "02.01.06 15:04",
    "02.01.06 15:04 -0700",
    "2.1.06 15:04:05",
    "2.1.06 15:04:05 -0700",
    "2.1.06 15:04",
    "2.1.06 15:04 -0700",
    "2.1.2006 15:04:05",
    "2.1.2006 15:04:05 -0700",
    "2.1.2006 15:04",
    "2.1.2006 15:04 -0700",
    "02.1.2006 15:04:05",
    "02.1.2006 15:04:05 -0700",
    "02.1.2006 15:04",
    "02.1.2006 15:04 -0700",
    "2.01.2006 15:04:05",
    "2.01.2006 15:04:05 -0700",
    "2.01.2006 15:04",
    "2.01.2006 15:04 -0700",
    "02.1.06 15:04:05",
    "02.1.06 15:04:05 -0700",
    "02.1.06 15:04",
    "02.1.06 15:04 -0700",
    "2.01.06 15:04:05",
    "2.01.06 15:04:05 -0700",
    "2.01.06 15:04",
    "2.01.06 15:04 -0700"}

// function returning a UNIX timestamp relative to the current time
func parseRelativeTime(rtime string) (int64, error) {

    rtime = rtime[1:]

    var secBackwards int64 = 0

    // iterate over different time chunks to get the days, hours and minutes
    for _, chunk := range strings.Split(rtime, ":") {
        var err error

        if len(chunk) == 0 {
            return 0, errors.New("incorrect relative time specification")
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
        default:
            return 0, errors.New("incorrect relative time specification")
        }
    }

    return (time.Now().Unix() - secBackwards), nil

}

// Entry point for external calls -------------------------------------------------
func ParseTimeArgument(timeString string) (int64, error) {
    var (
        err  error
        rerr error
        t    time.Time
        tRel int64
    )

    // incorporate location information
    loc, locerr := time.LoadLocation("Local")
    if locerr != nil {
        return int64(0), locerr
    }

    // check whether a relative timestamp was specified
    if timeString[0] == '-' {
        if tRel, rerr = parseRelativeTime(timeString); rerr == nil {
            return tRel, rerr
        } else {
            return int64(0), rerr
        }
    }

    // try to interpret string as unix timestamp
    if i, er := strconv.ParseInt(timeString, 10, 64); er == nil {
        return i, er
    }

    // then check other time formats
    for _, tFormat := range TimeFormats {
        t, err = time.ParseInLocation(tFormat, timeString, loc)
        if err == nil {
            return t.Unix(), err
        }
    }

    return int64(0), errors.New("Unable to parse time format")
}
