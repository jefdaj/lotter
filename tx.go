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
	"log"
	"regexp"
	"strings"
)

type Split struct {
	account string  // i.e. "Assets:Exchange:CoinFace"
	delta   *Amount // may be nil (left blank, to be calculated by ledger-cli)
	price   *Amount
	cost    *Amount
	line    string

	// if true, the delta has been calculated
	nullAmount bool

	comment string // needed???
}

// goal of this regexp is to match the whitespace between account name
// and amount.  Typically two (or more) spaces, or a single tab.
var accountSeparator = regexp.MustCompile(`\s{2,}|\t+`)

func parseSplit(line string) (Split, bool) {
	// bad variable names ahead... "...Split" refers to result of
	// strings.Split() as opposed to ledger-cli "splits"

	this := Split{line: line}

	commentSplit := strings.SplitN(line, ";", 2)
	if len(commentSplit) > 1 {
		this.comment = commentSplit[1]
	}

	trimmed := strings.TrimSpace(commentSplit[0])
	if trimmed == commentSplit[0] || trimmed == "" {
		// doesn't start with a space, or is only a comment
		return this, false
	}

	accountSplit := accountSeparator.Split(trimmed, 2)
	this.account = strings.TrimSpace(accountSplit[0])

	if len(accountSplit) > 1 {
		priceSplit := strings.SplitN(accountSplit[1], "@@", 2) // actually cost, not price
		if len(priceSplit) == 2 {
			tmp, err := parseAmount(priceSplit[1])
			if err != nil {
				log.Panic(err)
			}
			this.cost = &tmp
		} else {
			priceSplit = strings.SplitN(accountSplit[1], "@", 2)
			if len(priceSplit) == 2 {
				tmp, err := parseAmount(priceSplit[1])
				if err != nil {
					log.Panic(err)
				}
				this.price = &tmp
			}
		}

		tmp, err := parseAmount(priceSplit[0])
		if err != nil {
			log.Panic(err)
		}
		this.delta = &tmp
	} else {
		this.nullAmount = true
	}

	return this, true
}

func (this *Split) Price() *Amount {
	if this.price == nil {
		if this.cost == nil {
			log.Panicf("cannot determine price of split: %q", this.line)
		}
		tmp := this.cost.ZeroClone()
		this.price = &tmp

		this.price.Quo(this.cost.Rat, this.delta.Rat)
	}
	return this.price
}

func (this *Split) Cost() *Amount {
	if this.cost == nil {
		if this.price == nil {
			log.Panicf("cannot determine cost of split: %q", this.line)
		}
		tmp := this.price.ZeroClone()
		this.cost = &tmp
		this.cost.Mul(this.price.Rat, this.delta.Rat)
	}
	return this.cost
}

// Tally returns the balance change implied by a split.  If the split
// has a cost/price, the amount returned is the cost.  Otherwise the
// amount returned is the delta.
func (this *Split) Tally() *Amount {
	if this.cost != nil || this.price != nil {
		cost := this.Cost()
		if cost.Sign() != this.delta.Sign() {
			tmp := cost.NegClone()
			cost = &tmp
		}
		return cost
	}
	return this.delta
}

// Inventory returns the negative of the delta. For double entry
// accounting where "inventory" offsets the delta.
func (this *Split) Inventory() Amount {
	return this.delta.NegClone()
}
