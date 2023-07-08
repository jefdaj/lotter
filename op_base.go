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

// Operation base
//
// Usage:
//
//    lotter [-base <currency>] -f <filename> base
//
// The base operation modifies transaction splits, converting costs
// and amounts into the _base_ currency.  This is intended to be a
// pre-processor for the **lot** operation, allowing trades to be
// accounted for in terms of the _base_ currency, even when the trades
// are for other currencies.
//
// This operation observes prices in the ledger file.  When a split
// has a cost expressed in a currency other than _base_, and a price
// conversion to _base_ is available on the same day as the
// transaction, this operation rewrites the transaction splits
// converting the original cost currency into the _base_.
//
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"src.d10.dev/command"
)

func init() {
	command.RegisterOperation(
		baseMain,
		"base",
		"base [-b=<begin date>]",
		"Convert price/cost information to base currency (using ledger-cli price data).",
	)
}

func baseMain() error {
	// define flags
	beginFlag := flag.String("b", "", "begin date")

	err := command.Parse()
	if err != nil {
		return err
	}

	// validate flags
	if base == "" {
		return errors.New("A base currency is required, i.e. `-base=USD`.")
	}

	var begin time.Time
	if *beginFlag != "" {
		begin, err = time.Parse("2006/01/02", *beginFlag)
		if err != nil {
			command.Check(fmt.Errorf("bad begin date (%q): %w", *beginFlag, err))
		}
	}

	// observe price information, if any
	priceHistory := make(map[string]*big.Rat)

	for scanner.Scan() {
		txLines := scanner.Lines()

		for _, line := range txLines.Line {
			// we're looking for, i.e. "P 2004/06/21 02:17:58 TWCUX 27.76 USD"
			// https://www.ledger-cli.org/3.0/doc/ledger3.html#Commodity-price-histories
			if strings.HasPrefix(line, "P ") {
				command.V(2).Info("\t", line) // debug
				seg := strings.SplitN(line, ";", 2)
				field := strings.Fields(seg[0])

				// support "P 2004/06/21 TWCUX 27.76 USD" by inserting a time
				if len(field) == 5 {
					field = append(field[:2+1], field[2:]...)
					field[2] = "00:00:00"
				}

				counterIdx, invert := -1, false
				if field[5] == string(base) {
					counterIdx, invert = 3, false
				} else if field[3] == string(base) {
					counterIdx, invert = 5, true
				} else {
					command.V(1).Infof("ignoring non-base price (%q)", line)
					continue
				}

				date, err := time.Parse("2006/01/02 15:04:05", strings.Join(field[1:3], " "))
				if err != nil {
					command.Check(fmt.Errorf("failed to parse historical price (%q): %w", line, err))
				}

				price, ok := new(big.Rat).SetString(field[4])
				if !ok {
					command.Check(fmt.Errorf("failed to parse historical price (%q)", line))
				}
				if invert {
					price.Inv(price)
				}

				key := historyKey(date, Asset(field[counterIdx]))
				old, ok := priceHistory[key]
				if ok {
					// TODO(dnc): round strings to proper precision
					command.V(1).Infof("updating price history (was %s, now %s)\n\t%s", old.FloatString(6), price.FloatString(6), line)
				}
				priceHistory[key] = price
			}
		} // end collect price history

		payee, payeeIndex := txLines.Payee()
		if payeeIndex == PayeeNotFound {
			// not a transaction (maybe a comment)
			writeLines(append(txLines.Line, "")) // with a blank
			continue
		}
		if begin.After(txLines.Date) {
			writeLines(append(txLines.Line, "")) // with a blank
			continue
		}

		command.V(2).Info("\t", payee) // debug

		// prepare to display multiple errors
		var errs []error

		// first pass, find conversions to base
		conversion := make(map[string]Amount)
		for _, line := range txLines.Line[payeeIndex+1:] {
			split, ok := parseSplit(line)
			if !ok {
				if !strings.HasPrefix(strings.TrimLeft(line, " \t"), ";") { // check comment
					command.Check(fmt.Errorf("failed to parse transaction split: %q", line))
				}
				continue // comment is noop
			}

			if split.cost == nil && split.price == nil {
				// no price or cost to convert
				continue
			}

			cost := split.Cost()
			if cost == nil || cost.Asset == base {
				continue
			}

			// here we have a cost that must be converted into base currency

			key := historyKey(txLines.Date, cost.Asset)
			price, ok := priceHistory[key]
			if ok {
				// conversion based on cost
				tmp := new(big.Rat).Mul(price, cost.Rat)
				basis := NewAmount(base, *tmp)
				conversion[cost.String()] = basis
			} else {
				// alternately, convert based on delta
				key = historyKey(txLines.Date, split.delta.Asset)
				price, ok = priceHistory[key]
				if ok {
					tmp := new(big.Rat).Mul(price, split.delta.Rat)
					basis := NewAmount(base, *tmp.Abs(tmp))
					conversion[cost.String()] = basis
				} else {
					errs = append(errs, fmt.Errorf("missing price of %s or %s on %s", cost.Asset, split.delta.Asset, txLines.Date.Format("2006/01/02")))
				}
			}

		} // end first pass

		if len(conversion) > 0 {
			// second pass, alter
			for index, line := range txLines.Line[payeeIndex+1:] {
				split, ok := parseSplit(line)
				if !ok {
					continue // comment is noop
				}

				if split.cost != nil || split.price != nil {
					basis, ok := conversion[split.Cost().String()]
					basis = basis.AbsClone()
					if ok {
						// replace existing cost/price with basis
						txLines.Line[payeeIndex+1+index] = strings.Replace(line, "@", fmt.Sprintf("@@ %s ; @", basis), 1)
					}
				} else if split.delta != nil {
					deltaStr := split.delta.NegClone().String()
					basis, ok := conversion[deltaStr]
					if ok {
						// add basis where there may be no price, here we expect "<amount><space><asset>"
						field := strings.Fields(line)
						txLines.Line[payeeIndex+1+index] = strings.Replace(line, fmt.Sprintf("%s %s", field[1], field[2]), fmt.Sprintf("%s @@ %s ; ", split.delta, basis), 1)
						// sanity
						if txLines.Line[payeeIndex+1+index] == line {
							log.Panicf("failed to replace %q in line (%q)", deltaStr, line)
						}
					} else {
						// troubleshoot
						for key, _ := range conversion {
							log.Println("conversion available:", key)
						}
						log.Panicf("failed to convert %q to base currency", deltaStr)
					}
				}

			} // end second pass
		}

		// write txLines (which may have been modified above)
		writeLines(txLines.Line)
		for _, err = range errs {
			command.Error(err)
			fmt.Println("    FIXME:lotter base:  ", err) // write error to ledger data
		}

		fmt.Println("") // blank line between transactions

	} // end scan loop

	return nil
}

func historyKey(date time.Time, asset Asset) string {
	return fmt.Sprintf("%s %s", date.Format("2006/01/02"), asset)
}

