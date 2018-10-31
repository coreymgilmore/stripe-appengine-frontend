/*
Package timestamps is a simple library that creates timestamps and datetimes in a specific format.

Its use is two fold:
1) Simplify code since a one liner can now be used instead of the code blocks below (very minimal difference, I know).
2) Reduce formatting mistakes since calls to these func will always return a datetime or timestamp with the same format.
*/
package timestamps

import "time"

//Unix returns the number of seconds as an integer since epoch
func Unix() int {
	t := time.Now()
	s := t.Unix()
	return int(s)
}

//ISO8601 returns a datetime with the format YY-MM-DDTHH:MM:SS.mmmZ
func ISO8601() string {
	t := time.Now().UTC()
	s := t.Format("2006-01-02T15:04:05.000Z")
	return s
}
