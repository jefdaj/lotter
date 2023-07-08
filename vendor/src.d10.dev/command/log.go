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

// Log Helpers
//
// Logging API is a simple addition to Go stdlib log package.
//
// The V() helper limits verbosity.  For example,
//
//    command.V(2).Log("hello world")
//
// will write only if the "-v" flag appears twice (or more) in the
// command flags.
//
// If command.Error() or command.Errorf() is called, then
// command.Exit() will terminate with a non-zero status.
package command

import "log"

func Error(args ...interface{}) {
	log.Println(args...)
	if status == 0 {
		status = 1
	}
}

func Errorf(format string, args ...interface{}) {
	log.Printf(format, args...)
	if status == 0 {
		status = 1
	}
}

