// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ics provides support for reading Apple's iCalendar file format.
package ics

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

type Calendar struct {
	Event []*Event
}

type Event struct {
	UID                            string
	Start, End                     time.Time
	Summary, Location, Description string
}

func (e *Event) String() string {
	s := make([]string, 0, 6)
	s = append(s, "UID:"+e.UID)
	s = append(s, "Start: "+e.Start.String())
	s = append(s, "End: "+e.End.String())
	s = append(s, "Summary: "+e.Summary)
	s = append(s, "Location: "+e.Location)
	s = append(s, "Description: "+e.Description)
	return strings.Join(s, "\n")
}

func Decode(rd io.Reader) (c *Calendar, err error) {
	return decode(rd, true)
}

func DecodePreserveCRLF(rd io.Reader) (c *Calendar, err error) {
	return decode(rd, false)
}

func decode(rd io.Reader, removeCRLF bool) (c *Calendar, err error) {
	r := bufio.NewReader(rd)
	for {
		key, value, err := decodeLine(r, removeCRLF)
		if err != nil {
			return nil, err
		}
		if key == "BEGIN" {
			if c == nil {
				if value != "VCALENDAR" {
					return nil, errors.New("didn't find BEGIN:VCALENDAR")
				}
				c = new(Calendar)
			}
			if value == "VEVENT" {
				e, err := decodeEvent(r, removeCRLF)
				if err != nil {
					return nil, err
				}
				c.Event = append(c.Event, e)
			}
		}
		if key == "END" && value == "VCALENDAR" {
			break
		}
	}
	sort.Sort(eventList(c.Event))
	return c, nil
}

func decodeEvent(r *bufio.Reader, removeCRLF bool) (*Event, error) {
	e := new(Event)
	var key, value string
	var err error
	for {
		if err != nil {
			if err == io.EOF {
				return e, nil
			}
			return nil, err
		}
		key, value, err = decodeLine(r, removeCRLF)
		// Fix dates
		if len(key) >= 7 && key[0:7] == "DTSTART" {
			key = "DTSTART"
		}
		if len(key) >= 5 && key[0:5] == "DTEND" {
			key = "DTEND"
		}
		value = UnescapeText(value)
		switch key {
		case "END":
			if value != "VEVENT" {
				// Temporary ignore any other END. Problems with END:VALARM found.
				// return nil, errors.New("unexpected END value")
				continue

			} else {
				return e, nil
			}
		case "UID":
			e.UID = value
		case "DTSTART":
			e.Start, err = decodeDate(value)
		case "DTSTART;VALUE=DATE":
			e.Start, err = decodeDate(value)
		case "DTEND":
			e.End, err = decodeDate(value)
		case "DTEND;VALUE=DATE":
			e.End, err = decodeDate(value)
		case "SUMMARY":
			e.Summary = value
		case "LOCATION":
			e.Location = value
		case "DESCRIPTION":
			e.Description = value
		}
	}
}

func decodeTime(value string) (time.Time, error) {
	const layout = "20060102T150405Z"
	return time.Parse(layout, value)
}

func decodeDate(value string) (time.Time, error) {
	const layout = "20060102"
	if len(value) < 8 {
		return time.Time{}, nil //Devolvemos fecha por defecto
	}
	return time.Parse(layout, value[0:8])
}

func decodeLine(r *bufio.Reader, removeCRLF bool) (key, value string, err error) {
	var buf bytes.Buffer
	done := false
	for !done {
		// get full line
		//		b, isPrefix, err := r.ReadLine()
		b, err := r.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				done = true
			} else {
				return "", "", err
			}
		}
		if len(b) == 0 {
			//			return "", "", errors.New("unexpected blank line")
			continue
		}
		if b[0] == ' ' {
			b = b[1:]
		}
		buf.Write(b)

		b, err = r.Peek(1)
		if err != nil || b[0] != ' ' {
			break
		}
	}
	p := strings.SplitN(buf.String(), ":", 2)
	if len(p) != 2 {
		fmt.Println("ERROR: len(p)=", len(p), p)
		return "", "", errors.New("bad line, couldn't find key:value")
	}
	if !removeCRLF {
		trimmed1 := strings.Trim(p[0], " ")
		trimmed2 := strings.Trim(p[1], " ")
		return trimmed1, trimmed2, nil
	}
	trimmed1 := strings.Trim(p[0], " \r\n")
	trimmed2 := strings.Trim(p[1], " \r\n")
	return trimmed1, trimmed2, nil
}

type eventList []*Event

func (l eventList) Less(i, j int) bool {
	if l[i].Start.IsZero() {
		return true
	}
	if l[j].Start.IsZero() {
		return false
	}
	return l[i].Start.Before(l[j].Start)
}
func (l eventList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l eventList) Len() int      { return len(l) }

// From https://github.com/laurent22/ical-go/blob/master/ical.go
func UnescapeText(s string) string {
	s = strings.Replace(s, "\\;", ";", -1)
	s = strings.Replace(s, "\\,", ",", -1)
	s = strings.Replace(s, "\\n", "\n", -1)
	s = strings.Replace(s, "\\\\", "\\", -1)
	s = strings.Replace(s, "\n", " ", -1)
	s = strings.Replace(s, "&nbsp", " ", -1)
	return s
}
