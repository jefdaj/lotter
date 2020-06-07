// Copyright (C) 2019  David N. Cohen

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"bufio"
	"io"
	"strings"
	"time"
)

const PayeeNotFound int = -1

// "TxLines" has become a misnomer, as Line array might contain any
// ledger-cli data, even comments.  The payee index will be set only
// when lines contain a transaction with a payee line.
type TxLines struct {
	Line  []string
	payee *int      // index
	Date  time.Time // based on date in payee line
}

// Inspect transaction lines and find the "payee" line.  The payee
// line preceeds the "splits", it starts with a date.
func (this *TxLines) Payee() (string, int) {
	if this.payee == nil {
		this.findPayee()
	}
	if *this.payee < 0 {
		return "", PayeeNotFound
	} else {
		return this.Line[*this.payee], *this.payee
	}
}

// helper to get *int
func newInt(x int) *int { return &x }

var dateFormat = [...]string{
	"2006/1/_2",
	"2006-1-_2",
}

// Parse a date, the first part of payee line.  This wrapper around
// time.Parse attempts multiple date formats.
func parseDate(str string) (t time.Time, e error) {
	for _, f := range dateFormat {
		t, e = time.Parse(f, str)
		if e == nil {
			break
		} else {
			// troubleshoot
			//log.Printf("%q is not a date (%q)", str, f)
		}
	}
	return
}

// returns offset of payee line, or -1 if not a transaction.
func (this *TxLines) findPayee() int {
	isTx := false
	for i := len(this.Line) - 1; i >= 0; i-- {
		splitComment := strings.Split(this.Line[i], ";")
		trimmed := strings.TrimLeft(splitComment[0], "\t ")
		//log.Printf("i = %d; trimmed = %q", i, trimmed) // troubleshoot
		if trimmed != splitComment[0] {
			// leading space indicates a row of the transaction
			if trimmed != "" {
				isTx = true
			}
			continue
		}

		if !isTx {
			this.payee = newInt(-1)
			break
		}

		var err error
		// The line immediately preceeding the deltas is the payee
		splitSpace := strings.Split(splitComment[0], " ")
		this.Date, err = parseDate(splitSpace[0])
		if err == nil {
			// line starts with a date, good enough for us
			this.payee = newInt(i)
			break
		} else {
			//log.Printf("trouble payee line (%q): %s", this.Line[i], err) // troubleshoot
			this.payee = newInt(-1)
			break
		}
	}
	return *this.payee
}

func (this *TxLines) Len() int { return len(this.Line) }

type TxScanner struct {
	scanner *bufio.Scanner
	lines   TxLines
}

func NewTxScanner(in io.Reader) *TxScanner {
	this := &TxScanner{
		scanner: bufio.NewScanner(in),
	}
	return this
}

func (this *TxScanner) Scan() bool {
	nonEmpty := false
	this.lines = TxLines{Line: make([]string, 0)}
	for this.scanner.Scan() {
		line := this.scanner.Text()

		if strings.TrimSpace(line) == "" {
			if nonEmpty {
				// we've reached the end of a tx
				break
			}
		}

		this.lines.Line = append(this.lines.Line, line)

		split := strings.Split(line, ";")
		if strings.TrimSpace(split[0]) != "" {
			// non empty, non comment
			nonEmpty = true
		}

	}
	return this.lines.Len() > 0
}

func (this *TxScanner) Lines() TxLines { return this.lines }

func (this *TxScanner) Err() error { return this.scanner.Err() }
