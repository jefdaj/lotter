// COPYRIGHT(C) 2018-2020  David N. Cohen.
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
	"fmt"
	"log"
)

type verbose bool

func V(threshold int) verbose { return verbose(threshold <= int(verboseFlag)) }

func (this verbose) Println(args ...interface{}) (int, error) {
	if this {
		return fmt.Println(args...)
	}
	return 0, nil
}
func (this verbose) Printf(format string, args ...interface{}) (int, error) {
	if this {
		return fmt.Printf(format, args...)
	}
	return 0, nil
}

func (this verbose) Log(args ...interface{}) {
	if this {
		log.Println(args...)
	}
}

func (this verbose) Logf(format string, args ...interface{}) {
	if this {
		log.Printf(format, args...)
	}
}

// deprecated "Info" helpers
func (this verbose) Info(args ...interface{})                 { this.Log(args...) }
func (this verbose) Infof(format string, args ...interface{}) { this.Logf(format, args...) }
