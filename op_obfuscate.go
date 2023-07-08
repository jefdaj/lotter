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
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"strings"

	"src.d10.dev/command"
)

func init() {
	command.RegisterOperation(
		obfuscateMain,
		"obfuscate",
		"obfuscate [-prune=<int>] [-salt=<string>]",
		"Convert account names, concealing potentially sensitive data.",
	)
}

func obfuscateMain() error {
	// define flags
	pruneFlag := flag.Int("prune", 1, "name depth where obfuscation begins")
	saltFlag := flag.String("salt", "", "make obfuscation hashes unique and reproducable only when salt is known")

	err := command.Parse()
	if err != nil {
		return err
	}

	for scanner.Scan() {
		txLines := scanner.Lines()

		line, index := txLines.Payee()
		if index != PayeeNotFound {
			// obfuscate the transaction name
			commentPart := strings.SplitN(line, ";", 2)
			spacePart := strings.SplitN(commentPart[0], " ", 2)
			h := sha256.Sum256([]byte(spacePart[1] + *saltFlag))
			spacePart[1] = hex.EncodeToString(h[:8])
			// put original line in a comment above the obfuscated line
			txLines.Line[index] = fmt.Sprintf("; %s\n%s %s \t; %s", line, spacePart[0], spacePart[1], "")
		}

		for index, line := range txLines.Line {

			// TODO(dnc): may need to remove or obfuscate comments,
			// especially trailing comments which ledger exports to CSV.

			split, ok := parseSplit(line)
			if !ok {
				continue
			}

			// The obfuscated account name should have the same number of
			// parts as the original, and strings that appear in the
			// original should always map to the same obfuscated name.  This
			// allows the `lot` operation to be run after `obfuscate`.

			// "Pruned" parts at the start of the name are not obfuscated.
			// This allows human readable "Assets" vs "Expenses", common
			// ledger-cli conventions.

			cleartext := strings.Trim(split.account, "[]")
			parts := strings.Split(cleartext, ":")
			for n := len(parts); n > *pruneFlag; n-- {
				h := sha256.Sum256([]byte(parts[n-1] + *saltFlag))
				parts[n-1] = hex.EncodeToString(h[:3]) // TODO(dnc): make length configurable
			}
			obfuscated := strings.Join(parts, ":")

			txLines.Line[index] = strings.Replace(line, cleartext, obfuscated, 1)
		}
		writeLines(txLines.Line)
		fmt.Println("") // blank line between transactions
	} // end scan loop
	return nil
} // end obfuscateMain
