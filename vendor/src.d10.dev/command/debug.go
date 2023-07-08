// +build debug

// Copyright (C) 2018-2020  David N. Cohen
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

package command

import (
	"log"
)

// Check logs an error and exits with non-zero status, when error is
// not nil. The debug version of Check panics, in order to show a
// stack trace.
func Check(err error) {
	if err != nil {
		log.Panic(err) // produce stack trace
	}
}
