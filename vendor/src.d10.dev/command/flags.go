// Copyright (C) 2018,2019  David N. Cohen
// This file is part of src.d10.dev/command
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Flag Helpers
//
// Flags support provided by the golang standard flags package. The
// command package defines several types to support more advanced flag
// features.
//
// See StringSet, BoolMap, and BoolCount.
package command

import (
	"fmt"
)

/* XXX
// Type FlagSet wraps flag.FlagSet.  This allows FlagSet.Parse to
// signal (return an error) to an operation running in help mode.
type FlagSet struct {
	flag.FlagSet
}

func (this *FlagSet) Parse(arguments []string) error {
	if helpMode {
		return this.FlagSet.Parse([]string{"-h"}) // flag.FlagSet returns flag.ErrHelp
	}
	return this.FlagSet.Parse(arguments)
}
*/

// A flag of type StringSet can be repeated on the command line. For
// example: `command -user=alice -user=bob -user=carol` gives the flag
// value []string{"alice", "bob", "carol"}
type StringSet []string

// https://stackoverflow.com/questions/28322997/how-to-get-a-list-of-values-into-a-flag-in-golang

func (this *StringSet) String() string {
	return fmt.Sprintf("StringSet%s", []string(*this))
}

func (this *StringSet) Set(value string) error {
	*this = append(*this, value)
	return nil
}

func (this *StringSet) Strings() []string {
	return []string(*this)
}

// A flag of type BoolMap can be repeated on the command line.  For
// example, "command -user=alice -user=carol" gives the value
// {"alice": true, "carol": true}
type BoolMap map[string]bool

func (this BoolMap) String() string {
	return fmt.Sprintf("StringMap%#v", this)
}
func (this BoolMap) Set(value string) error {
	this[value] = true
	return nil
}

// A flag of type BoolCount can be repeated on the command line.  For
// example: "command -v -v -v" gives the flag value 3.
type BoolCount int

func (this *BoolCount) String() string {
	return fmt.Sprintf("BoolCount{%d}", int(*this))
}
func (this *BoolCount) Set(value string) error {
	if value == "true" {
		*this++
	}
	return nil
}
func (this *BoolCount) IsBoolFlag() bool { return true }
