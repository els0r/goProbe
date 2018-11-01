/////////////////////////////////////////////////////////////////////////////////
//
// DBTime_test.go
//
// Testing wrapper for time parsing functions
//
// Written by Lennart Elsen lel@open.ch, June 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import (
    "testing"
    "time"
)

func TestTimeFormatParsing(t *testing.T) {
    // incorporate location information
    loc, locerr := time.LoadLocation("Local")
    if locerr != nil {
        t.Fatalf("failed to load location: %s", locerr.Error())
    }

    var TestDate = time.Date(2007, time.September, 25, 14, 23, 00, 0, loc)

    for _, format := range TimeFormats {
        // get the date string in the current format to be tested
        dateString := TestDate.Format(format)

        // parse the time using the Parse function and compare the retrieved timestamp
        tstamp, err := ParseTimeArgument(dateString)
        if err != nil {
            t.Fatalf("failed to parse date '%s': %s", dateString, err.Error())
        }
        if tstamp != TestDate.Unix() {
            t.Fatalf("parser got unix timestamp: '%d'; expected: '%d'", tstamp, TestDate.Unix())
        }
    }
}
