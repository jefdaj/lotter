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

// Operation: Lot
//
//    usage: lotter -f <filename> lot
//
// The `lot` operation adds "splits" to transactions, representing lot
// inventory, cost basis, and gains.
//
// Each lot is a `ledger-cli` "account", named by convention with
// prefix "Lot", followed by the date the lot was created, and
// inventory and cost information.  This naming convention is intended
// to provide unique lot names.  (It could fail to do so, if multiple
// purchases occur on the same day, for the same amount and cost.)
//
// `lotter` considers a transaction to be a purchase when it finds a
// split for a positive amount, with cost information associated with
// it.  When constructing your ledger entries, use for example "100
// ABC @ 0.02 USD" or "100 ABC @@ 2 USD".
//
// Similarly, `lotter` considers a transaction to be a sale when the
// amount is negative and has a cost associated.  To these
// transactions, `lotter` adds splits that "consume" inventory (and
// basis) acquired earlier.
//
// To see options available, run `lotter help lot`.
//
package main

import (
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"src.d10.dev/command"
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     lotMain,
		Name:        "lot",
		Syntax:      "lot [-prune=<int>]",
		Description: "Add inventory, basis, and gain splits to ledger-cli data.",
	})
}

// simple output helper
func writeLines(lines []string) {
	for _, line := range lines {
		fmt.Println(line)
	}
}

var (
	// command line flags
	pruneFlag *int
	orderFlag *string

	// indexes to the lot queue are a qualifier and an asset
	// qualifier is non-empty when lots are per-account (not just per-asset)
	lotQueue = make(map[Asset]map[string]LotQueue)
)

func lotMain() error {

	// define flags
	pruneFlag = command.OperationFlagSet.Int("prune", 0, "name depth of account-specific lots") // TODO(dnc): document prune (maybe rename)
	orderFlag = command.OperationFlagSet.String("order", "fifo", "order in which lot inventory is consumed, may be fifo or lifo")

	err := command.ParseOperationFlagSet()
	if err != nil {
		return err
	}

	// validate flags
	if base == "" {
		return errors.New("A base currency is required, i.e. `-base=USD`.")
	}

	// prepare to add lot splits to ledger data
	writer := tabwriter.NewWriter(os.Stdout, 4, 8, 0, '\t', 0)

	for scanner.Scan() {

		txLines := scanner.Lines()

		payee, payeeIndex := txLines.Payee()
		if payeeIndex == PayeeNotFound {
			// not a transaction (maybe a comment)
			writeLines(append(txLines.Line, "")) // with a blank
			continue
		}

		command.V(1).Info("transaction:\n\t", payee)

		// keep track of lots affected by this transaction
		var lot []Lot
		var inventory []Amount
		var basis []Amount
		var comment []string
		// (original intent was to track moves and trades both in each transaction; however currently we treat each transaction as either a move or trades, not both)

		splits, isTrade, _, err := produceSplits(txLines.Line[payeeIndex+1:])
		if err != nil {
			writeLines(txLines.Line)
			log.Printf("\nFailed to process transaction (%q):\n\t", payee)
			log.Println(err)
			os.Exit(1)
		}

		if !isTrade {
			// Moves are splits without a price/cost associated (i.e. moving
			// an asset from a hot wallet to a cold wallet)

			// tally moves by qualifier
			moves := produceMoves(splits)

			l, i, b, c, err := consumeMoves(moves)
			if err != nil {
				writeLines(txLines.Line)
				log.Printf("Failed to process move transaction (%q):", payee)
				log.Println("\t", err)
				os.Exit(1)
			}
			lot = append(lot, l...)
			inventory = append(inventory, i...)
			basis = append(basis, b...)
			comment = append(comment, c...)
		} else {
			l, i, b, c, err := consumeTrades(splits, txLines.Date)
			if err != nil {
				writeLines(txLines.Line)
				log.Printf("Failed to process trade transaction (%q):", payee)
				log.Println("\t", err)
				os.Exit(1)
			}
			lot = append(lot, l...)
			inventory = append(inventory, i...)
			basis = append(basis, b...)
			comment = append(comment, c...)
		}

		// sanity check that inventory, lot, basis, comment arrays have equal length
		if len(lot) != len(inventory) || len(lot) != len(basis) || len(lot) != len(comment) {
			log.Panic("mismatch of lot/inventory/basis changes")
		}

		// Before writing original splits, we comment out the price/cost
		// portion of the split.  That information is now expressed in lot
		// basis and/or gains.
		for i, line := range txLines.Line[payeeIndex+1:] {
			priceIndex := strings.IndexByte(line, '@')
			if priceIndex != -1 {
				commentIndex := strings.IndexByte(line, ';')
				if commentIndex == -1 || commentIndex > priceIndex {
					// comment out price/cost
					_ = i
					txLines.Line[payeeIndex+1+i] = strings.Replace(line, "@", "; @", 1)
				}
			}
		}

		// write lot inventory and basis splits
		for i, _ := range inventory {
			// compose a more verbose comment
			var verbose string
			switch inventory[i].Sign() {
			case 0:
				log.Panicf("zero inventory! %q", payee)
			case 1:
				// positive inventory means lot consumed
				verbose = fmt.Sprintf("%s (inventory consumed)", comment[i])
			case -1:
				verbose = fmt.Sprintf("%s (inventory)", comment[i])
			}
			fmt.Fprintf(writer, "    [%s]\t\t%s \t; %s\n", lot[i].name, inventory[i].String(), verbose)
			switch basis[i].Sign() {
			case 0:
				verbose = fmt.Sprintf("%s (basis unchanged)", comment[i])
			case 1:
				// positive basis means inventory added
				verbose = fmt.Sprintf("%s (basis)", comment[i])
			case -1:
				verbose = fmt.Sprintf("%s (basis consumed)", comment[i])
			}
			if basis[i].Sign() == 0 {
				// comment out 0 basis
				fmt.Fprintf(writer, "    ;[%s]\t\t%s \t; %s\n", lot[i].name, basis[i].String(), verbose)
			} else {
				fmt.Fprintf(writer, "    [%s]\t\t%s \t; %s\n", lot[i].name, basis[i].String(), verbose)
			}

		}

		// tally whether gains are long or short term
		// note that we tally the rendered amounts, which may be rounded
		longBasis := new(big.Rat)
		shortBasis := new(big.Rat)
		var longInventory, shortInventory *Amount

		totalGain := new(big.Rat) // positive indicates sell, negative indicates buy
		if isTrade {
			for _, qualified := range splits {
				for _, split := range qualified {
					for _, s := range split {
						if s.delta.Asset == base {
							printed, ok := new(big.Rat).SetString(s.delta.FloatString())
							if !ok {
								log.Panicf("bad amount %s", s.delta)
							}
							totalGain.Add(totalGain, printed)
						}
					}
				}
			}
		}
		for i, _ := range inventory {

			var isLongTerm, isShortTerm bool
			if inventory[i].Sign() > 0 { // double-entry, positive inventory indicates sell
				// in U.S.A, distinguish long term gain/loss from short term
				_, years, _, _, _, _, _, _ := Elapsed(lot[i].date, txLines.Date)
				if years > 0 {
					isLongTerm = true
				} else {
					isShortTerm = true
				}

				if longInventory == nil {
					tmp := inventory[i].ZeroClone()
					longInventory = &tmp
					tmp2 := inventory[i].ZeroClone()
					shortInventory = &tmp2
					// TODO(dnc): if `tmp = ` instead of `tmp2 := ` above, longInventory and shortInventory end up the same pointer!  investigate why.
					// sanity
					if fmt.Sprintf("%p", shortInventory) == fmt.Sprintf("%p", longInventory) {
						log.Panic("longInventory and shortInventory are same pointer")
					}
				}

				// sanity check, if fails inventory tally must be map[Asset]*Amount
				if longInventory.Asset != inventory[i].Asset {
					log.Panicf("trade with mixed inventory (%s and %s)", longInventory.Asset, inventory[i].Asset)
				}

			}

			printed, ok := new(big.Rat).SetString(basis[i].FloatString())
			if !ok {
				log.Panicf("bad amount (%q)", basis[i])
			}
			if isLongTerm {
				longBasis.Add(longBasis, printed)
				longInventory.Add(longInventory.Rat, inventory[i].Rat)
			}
			if isShortTerm {
				shortBasis.Add(shortBasis, printed)
				shortInventory.Add(shortInventory.Rat, inventory[i].Rat)
			}
			totalGain.Add(totalGain, printed)
		} // end inventory loop

		if shortInventory != nil && longInventory != nil {
			sellInventory := new(big.Rat).Add(shortInventory.Rat, longInventory.Rat)

			// short term gain = (total gain) * (inventory consumed short term) / (total inventory consumed)
			shortTermGain := new(big.Rat)
			shortTermGain.Mul(totalGain, new(big.Rat).Quo(shortInventory.Rat, sellInventory))

			// long term gain = (total gain) - (short term gain)
			longTermGain := new(big.Rat).Sub(totalGain, shortTermGain)

			// finally add splits to represent gain or loss
			// note in ledger-cli gains are negative
			if shortTermGain.Sign() != 0 {
				shortTermGain.Neg(shortTermGain)
				fmt.Fprintf(writer, "    [Lot:Income:short term gain]\t\t %s \t; :GAIN:SHORTTERM: \n", NewAmount(base, *shortTermGain))
			}
			if longTermGain.Sign() != 0 {
				longTermGain.Neg(longTermGain)
				fmt.Fprintf(writer, "    [Lot:Income:long term gain]\t\t %s \t; :GAIN:LONGTERM: \n", NewAmount(base, *longTermGain))
			}
		} // end if sale

		// output
		writeLines(txLines.Line)
		writer.Flush()
		fmt.Println("") // blank between transactions (truncated by Scan())
	} // end txScan loop

	return nil
}

func getQueue(asset Asset, qualifier string) LotQueue {
	// sanity check
	if asset == base {
		log.Printf("getQueue(%q): base currency requested!", asset)
	}

	_, ok := lotQueue[asset]
	if !ok {
		lotQueue[asset] = make(map[string]LotQueue)
	}
	_, ok = lotQueue[asset][qualifier]
	if !ok {
		lotQueue[asset][qualifier] = LotQueue{order: order(*orderFlag)}
	}

	// sanity check
	if asset == base && lotQueue[asset][qualifier].Len() > 0 {
		log.Panicf("getQueue(%q): base currency has lots!", asset)
	}

	return lotQueue[asset][qualifier]
}

func buy(lot Lot, qualifier string) {
	queue := getQueue(lot.inventory.Asset, qualifier)
	queue.Buy(lot)
	lotQueue[lot.inventory.Asset][qualifier] = queue // store change made by queue.Buy()
}

func sell(qualifier string, delta Amount) (lot []Lot, inventory []Amount, basis []Amount, err error) {
	if delta.Asset == base {
		err = fmt.Errorf("attempt to sell base asset (%s)", delta.String())
		return
	}

	queue := getQueue(delta.Asset, qualifier)
	if queue.Len() < 1 {
		err = fmt.Errorf("attempt to sell (%s) from empty lot (%q[%s])", delta.String(), delta.Asset, qualifier)
		return
	}
	lot, inventory, basis, err = queue.Sell(delta)
	if err != nil {
		return
	}
	if len(lot) != len(inventory) || len(inventory) != len(basis) {
		err = fmt.Errorf("sell lot count mismatch! (%d vs %d vs %d)", len(lot), len(inventory), len(basis)) // sanity
		return
	}
	lotQueue[delta.Asset][qualifier] = queue // store changes made by queue.Sell()
	return
}

func getAssetQualifier(split Split) string {

	qual := split.account
	if *pruneFlag > -1 {
		// prune account name
		accountSeg := strings.Split(split.account, ":")
		if len(accountSeg) > *pruneFlag {
			// pruneFlag <= 2 treats "Assets:BTC:hot" and "Assets:BTC:cold" as
			// the same lot queue.  pruneFlag >= 3 treats them as separate lot
			// queues.  Pruning at 0 treats all BTC in the same lot queue.
			qual = strings.Join(accountSeg[:*pruneFlag], ":")
		}
	}

	return qual
}

func produceMoves(splitSet map[Asset]map[string][]Split) map[Asset]map[string]*big.Rat {
	ret := make(map[Asset]map[string]*big.Rat)

	// tally per asset
	for asset, qualified := range splitSet {
		ret[asset] = make(map[string]*big.Rat)

		for qual, splits := range qualified {
			ret[asset][qual] = new(big.Rat)
			for _, split := range splits {
				if split.price != nil || split.cost != nil {
					// splits with cost associated are not "moves"
					continue
				}
				ret[asset][qual].Add(ret[asset][qual], split.delta.Rat)
			}
		}
	}
	return ret
}

/* non-trivial move example that consumeMoves must support:
2017/01/01 non-trivial move example
    Assets:Crypto:on-chain        -100.00 ABC ; consume 100 from source lot
    Assets:Crypto:exchange          79.90 ABC ; new lot has less than 100!
    Expenses:Crypto:exchange:fee              ; ledger-cli will calculate, we won't bother

note that to support transactions like this, we do not require that
splits offset.  We require that the source data has correct, non-null,
deltas!

TODO(dnc): support following.  probably strategy is 1st pass consume non-null amounts, then second pass to consume anything that remains

2017/01/05 example move sell side specified and fee
    Assets:Crypto:Exchange                        -1 XRP
    Assets:Crypto:Exchange                     -0.01 XRP
    Expenses:Crypto:Exchange:fee                0.01 XRP
    Assets:Crypto:RCL

			// We must tolerate null amounts!  Because `ledger print`
			// outputs null amounts even when the source data is explicit!

*/

func consumeMoves(moves map[Asset]map[string]*big.Rat) (lot []Lot, inventory []Amount, basis []Amount, comment []string, err error) {

	// Each move consumes inventory (like a sell) and creates
	// offsetting inventory (like a buy).  The date of the original
	// inventory should be preserved (so we don't go from long-term to
	// short-term gain), as should the original cost basis.

	tmpQueue := make(map[Asset]*LotQueue)

	for asset, qualified := range moves {
		if asset == base {
			// moves of base currency have no effect on lots
			continue
		}
		tmpQueue[asset] = &LotQueue{order: order(*orderFlag)}

		for qual, delta := range qualified {
			switch delta.Sign() {
			case 0:
				// offsetting splits net zero, noop
				continue
			case 1:
				// positive delta, new inventory
				// handle this side of move in second pass
			case -1:
				// negative delta, consume inventory
				amt := NewAmount(asset, *delta)
				l, i, b, e := sell(qual, amt)
				if e != nil {
					err = e
					return
				}
				for j, _ := range l {
					// prepare for output
					lot = append(lot, l[j])
					inventory = append(inventory, i[j].Clone())
					basis = append(basis, b[j].Clone())
					comment = append(comment, fmt.Sprintf(":MOVE: move %s from %s (%d of %d)", amt, qual, j+1, len(l)))

					// remember this inventory for second pass
					tmpLot := NewLot("tmp", l[j].date, i[j], b[j].NegClone())
					tmpQueue[asset].Buy(*tmpLot)
				}
			}

		} // end first pass

		for qual, delta := range qualified {
			switch delta.Sign() {
			case 0:
				// offsetting splits net zero, noop
				continue
			case 1:
				// positive delta, new inventory
				amt := NewAmount(asset, *delta).NegClone()
				l, i, b, e := tmpQueue[asset].Sell(amt)
				if e != nil {
					err = e
					return
				}
				for j, _ := range l {
					// the new lot should have same date as old lot, a
					// different quality, and inventory equaling the portion
					// sold.
					shortName := lotShortName(i[j], NewAmount(b[j].Asset, *l[j].price))
					name := fmt.Sprintf("Lot:%s:%s:%s:%d", qual, l[j].date.Format("2006-01-02"), shortName, l[j].weight)
					newLot := NewLot(name, l[j].date, i[j], b[j].NegClone())
					newLot.weight = l[j].weight // same date and weight as consumed inventory

					// new inventory
					buy(*newLot, qual)

					// prepare for output
					lot = append(lot, *newLot)
					inventory = append(inventory, i[j].NegClone())
					basis = append(basis, b[j].NegClone())
					comment = append(comment, fmt.Sprintf(":MOVE: move %s to %s", newLot.inventory, qual))
				}
			case -1:
				// negative delta, consumed in first pass
				continue
			}
		} // end second pass

	}
	return
}

// this function inspects the splits, organizes by asset and
// qualifier.  Returns true if trades are present (splits with
// cost/price), and another true if splits balance (no null-amount).
func produceSplits(splitLines []string) (ret map[Asset]map[string][]Split, isTrade bool, balanced bool, err error) {
	ret = make(map[Asset]map[string][]Split)
	tally := make(map[Asset]*big.Rat)

	var noDelta *Split // some transactions have a single split without delta

	for _, line := range splitLines {
		split, ok := parseSplit(line)
		if !ok {
			if !strings.HasPrefix(strings.TrimLeft(line, " \t"), ";") { // check comment
				err = fmt.Errorf("failed to parse transaction split: %q", line)
				return
			}
			continue // comment is noop
		}

		if split.delta == nil {
			// process null-amount split after all the others
			noDelta = &split
			continue
		}

		if split.price != nil || split.cost != nil {
			isTrade = true
		}

		qualifier := getAssetQualifier(split)

		// tally amounts
		t, ok := tally[split.Tally().Asset]
		if !ok {
			t = new(big.Rat)
		}
		t.Add(t, split.Tally().Rat)
		tally[split.Tally().Asset] = t

		// organize splits by asset
		_, ok = ret[split.Tally().Asset]
		if !ok {
			ret[split.Tally().Asset] = make(map[string][]Split)
		}
		_, ok = ret[split.Tally().Asset][qualifier]
		if !ok {
			ret[split.Tally().Asset][qualifier] = make([]Split, 0, 1)
		}

		ret[split.Tally().Asset][qualifier] = append(ret[split.Tally().Asset][qualifier], split)
	}

	// If there is a null-amount split, use tally to determine its implied amount.
	if noDelta != nil {
		for asset, t := range tally {
			if t.Sign() != 0 {
				amt := NewAmount(asset, *(new(big.Rat).Neg(t)))
				noDelta.delta = &amt
				command.V(2).Infof("calculated amount (%s) for split (%q)", noDelta.delta, noDelta.line)
				ret[asset][getAssetQualifier(*noDelta)] = append(ret[asset][getAssetQualifier(*noDelta)], *noDelta)
				break // there can be only one TODO(dnc) sanity check that there only one non-zero tally
			}
		}
	}

	balanced = (noDelta == nil)

	/* old way XXX

	// Consider the unbalanced split as part of trade, only if this
	// transaction has trades (as opposed to moves).  Note that
	// split.asset will be "" here.
	if len(ret) > 0 && noDelta != nil {
		qualifier := getAssetQualifier(*noDelta)
		ret[AssetUnknown] = make(map[string][]Split)
		ret[AssetUnknown][qualifier] = make([]Split, 1)
		ret[AssetUnknown][qualifier][0] = *noDelta
	}
	*/

	return
}

func consumeTrades(trades map[Asset]map[string][]Split, date time.Time) (lot []Lot, inventory []Amount, basis []Amount, comment []string, err error) {

	for _, qualified := range trades {
		for qual, splits := range qualified {
			for _, split := range splits {

				if split.delta == nil {
					// should not longer be reached
					log.Panic("unexpected null amount in consumeTrades()")

					// without an amount, ledger-cli will compute it
					// TODO(dnc): error here if other split cost is not in base asset
					continue
				}

				if split.delta.Asset == base {
					// sending base currency has no effect on lots
					// but we don't want to see prices in non-base currencies here.
					if split.price != nil || split.cost != nil {
						err = fmt.Errorf("Trade has price in non-base currency: %q", split.line)
					}
					continue
				}

				if split.delta.Sign() == -1 { // negative delta

					// the sell side of a transaction can omit price, because
					// the buy side should have it.  Unless selling for base currency.
					if split.price == nil && split.cost == nil {
						continue
					} else if split.Cost().Asset != base {
						err = fmt.Errorf("sell-side priced in non-base currency: %q", split.line)
					}

					// this split is the sell side of transaction, consume inventory
					l, i, b, e := sell(qual, *split.delta)
					if e != nil {
						err = fmt.Errorf("failed to consume sell side of trade (%q): %w", split.line, e)
						return
					}

					for j, _ := range l {
						lot = append(lot, l[j])
						inventory = append(inventory, i[j].Clone())
						basis = append(basis, b[j].Clone())
						comment = append(comment, ":SELL:")
					}

					// end if split.delta.Negative
				} else {
					// buy side of transaction, create a new lot

					// TODO(dnc): allow a filter for only "Assets:..." accounts

					// new lots require a cost basis
					if split.price == nil && split.cost == nil {
						err = fmt.Errorf("apparent trade has no price/cost: %q", split.line)
						return
					}

					command.V(1).Infof("creating lot of %s with cost basis %s", split.delta.String(), split.Price().String())

					// lot name convention; TODO(dnc): ledger allows single space in account name
					lotName := lotShortName(*split.delta, *split.Price())
					lotDate := date
					lotBasis := *split.Cost()
					lotComment := ":BUY:"

					if lotBasis.Asset != base {
						// deferred gain
						// me must consume existing inventory, to buy the new lot.
						// basis is the total basis of inventory consumed.

						l, i, b, e := sell(qual, split.Cost().NegClone())
						if e != nil {
							err = e
							return
						}

						// sanity
						if len(l) != len(i) || len(l) != len(b) {
							log.Panic("deferred sell sanity check failed")
						}

						lotBasis = b[0].ZeroClone() // prepare to tally basis

						for j, _ := range l {
							// prepare for output
							lot = append(lot, l[j])
							inventory = append(inventory, i[j].Clone())
							basis = append(basis, b[j].Clone())
							comment = append(comment, ":SELL:DEFER:")

							// To avoid rounding errors, tally basis as rendeded to strings.
							roundedBasis, ok := new(big.Rat).SetString(b[j].FloatString())
							if !ok {
								log.Panicf("bad amount: %s", b[j])
							}

							lotBasis.Sub(lotBasis.Rat, roundedBasis) // tally basis (subtract a negative)

							// for purposes of long-term vs short term, use the
							// latest date of the consumed inventory.
							lotDate = l[j].date
							// TODO(dnc): should deferred gains show date of this transaction, or date of earlier consumed lot?
						}

						// lot name indicates deferred basis
						lotName = fmt.Sprintf("%s@%s", lotName, strings.ReplaceAll(lotBasis.String(), " ", ""))
						lotComment = ":BUY:DEFER:"
					} // end deferred

					// new lot from trade

					// lot account naming convention
					l := NewLot("temp", date, *split.delta, lotBasis)
					l.name = fmt.Sprintf("Lot:%s:%s:%s:%d", qual, lotDate.Format("2006-01-02"), lotName, l.weight)
					buy(*l, qual)

					lot = append(lot, *l)
					inventory = append(inventory, split.Inventory().Clone())
					basis = append(basis, lotBasis.Clone())
					comment = append(comment, lotComment)
				}
			} // end splits loop
		} // end qualifier loop
	} // end trades loop
	return
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// i.e. "100BTC@123.45USD"
func lotShortName(inventory Amount, price Amount) string {
	return fmt.Sprintf("%s@%s",
		strings.ReplaceAll(inventory.String(), " ", ""),
		strings.ReplaceAll(price.String(), " ", ""),
	)
}
