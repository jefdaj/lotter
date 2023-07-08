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
	"os/user"
	"path/filepath"
)

// ConfigDir provides a reasonable default location where configuration files may be found.
func ConfigDir() string {
	usr, err := user.Current()
	if err != nil {
		return filepath.Join("", "var", Command.Name)
	}
	return filepath.Join(usr.HomeDir, ".config", Command.Name)
}
