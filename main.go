// Copyright (C) 2019-2020  David N. Cohen

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

// Lotter is a command-line tool that works with trade data in
// `ledger-cli` format.  While `ledger-cli` is a fantastic calculator
// for double-entry accounting, its support for lots, cost basis, and
// gains is rather limited.  This tool is meant to provide features
// which (to the best of my knowledge) `ledger-cli` does not provide
// on its own.
//
// To best understand `lotter`, it is recommended to first be familiar
// with
// [`ledger-cli`](https://www.ledger-cli.org/3.0/doc/ledger3.html).
// Also, read background articles ["Multiple Currencies with Currency
// Trading
// Accounts"](https://github.com/ledger/ledger/wiki/Multiple-currencies-with-currency-trading-accounts),
// and Peter Selinger's ["Tutorial on Multiple Currency
// Accounting"](https://www.mathstat.dal.ca/~selinger/accounting/tutorial.html).
//
// Use `lotter` by first entering trade information into `ledger-cli`.
// Run `lotter` to add "lot" information, which enables `ledger-cli`
// to calculate cost basis and gains.
//
// Simple Example
//
// Let's say you purchased a cryptocurrency (we'll call it ABC), when
// it cost 2 cents.  A `ledger-cli` entry could look like:
//
//    2016-01-01 Bought ABC
//        Assets:Crypto          100 ABC @ 0.02 USD
//        Equity:Cash
//
// Later, ABC trades at $1, and you sell some.  In `ledger-cli`:
//
//    2017-01-01 Sell some ABC
//        Assets:Crypto          -1 ABC @ 1 USD
//        Assets:Exchange
//
// The idea of `lotter` is to add "splits" to these ledger entries.
// The added information captures the cost basis when a "lot" is
// created, and gains (losses) when inventory from a lot is sold.
// After `lotter`, the ledger entries look like:
//
//     2016-01-01 Bought ABC
//         Assets:Crypto                               100 ABC ; @ 0.02 USD
//         Equity:Cash
//         [Lot::2016/01/01:100ABC@0.02USD]            -100 ABC        ; :BUY: (inventory)
//         [Lot::2016/01/01:100ABC@0.02USD]            2 USD           ; :BUY: (basis)
//
//     2017-01-01 Sell some ABC
//         Assets:Crypto                               -1 ABC ; @ 1 USD
//         Assets:Exchange
//         [Lot::2016/01/01:100ABC@0.02USD]            1 ABC           ; :SELL: (inventory consumed)
//         [Lot::2016/01/01:100ABC@0.02USD]            -0.02 USD       ; :SELL: (basis consumed)
//         [Lot:Income:long term gain]                 -0.98 USD       ; :GAIN:LONGTERM:
//
// If your wondering why the last line ("long term gain") shows a
// negative number, when the actual gain is a positive 98 cents,
// recall that in `ledger-cli`'s double-entry method, income is
// expressed in negative numbers while expenses are positive.
// Similarly in `lotter`, lot inventory and gain are negative numbers,
// cost basis is positive.  This follows `ledger-cli`'s rules, and
// makes `lotter`'s splits net zero.
//
// The transactions described above are in
// `testdata/simple.ledger`. To see the effects of `lotter` on these
// transactions, compare the normal use of `ledger-cli`,
//
//    ledger -f testdata/simple.ledger bal
//
// with the effects of `lotter`,
//
//    lotter -f testdata/simple.ledger lot | ledger -f - bal
//
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"src.d10.dev/command"
)

// `go get src.d10.dev/dumbdown`
//go:generate sh -c "go doc | dumbdown > README.md"

var (
	// operations will scan and process ledger data
	scanner *TxScanner

	// base asset is what cost basis and gains are tallied in
	base Asset
)

func main() {
	command.RegisterCommand(
		"lotter",
		"lotter -f <filename> <operation> [<flag> ...]",
		"Add virtual splits to ledger-cli files, representing \"lots\" of inventory, to better track gains and losses.",
		command.OptionVerbose, //command.OptionConfig
	)

	// define flags
	fFlag := flag.String("f", "", "file to parse, use '-' for stdin")
	baseFlag := flag.String("base", "USD", "asset used for cost basis and gains")

	err := command.Parse()
	if err != nil {
		command.CheckUsage(err)
	}

	// validate flags
	if *fFlag == "" {
		command.CheckUsage(errors.New("Use \"-f <filename>\" to specify ledger data file.  Or use \"-f -\" for stdin."))
	}

	var file *os.File
	if *fFlag == "-" {
		file = os.Stdin
	} else {
		file, err = os.Open(*fFlag)
		if err != nil {
			command.Check(fmt.Errorf("failed to open ledger file (%q): %w", *fFlag, err))
		}
		defer file.Close()
	}

	base = Asset(*baseFlag)

	scanner = NewTxScanner(file)

	// omit date from log entries (confusing because log also shows dates from payee lines)
	log.SetFlags(0)

	op := flag.Arg(0)
	if op == "" {
		op = "lot" // default operation
	}
	command.Operate(op)

	// check for errors parsing file
	command.Check(scanner.Err())

	command.Exit()
}

