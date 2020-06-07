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
	"fmt"
	"log"
	"math/big"
	"sort"
	"time"

	"src.d10.dev/command"
)

type Lot struct {
	name   string
	date   time.Time
	weight uint // order tie-break when dates are equal

	inventory Amount

	startInventory Amount
	startCost      Amount

	price *big.Rat
}

var weight uint // counter for each lot created

func NewLot(name string, date time.Time, inventory, basis Amount) *Lot {
	if inventory.Sign() < 1 {
		log.Panicf("lot must have positive inventory (%s)", inventory.String()) // sanity
	}
	if basis.Sign() < 0 {
		log.Panicf("lot must have non-negative basis (%s)", basis.String()) // sanity
	}

	price := new(big.Rat).Quo(basis.Rat, inventory.Rat) // price = (total cost) / (how many)

	weight++
	this := &Lot{
		name:           name,
		date:           date,
		weight:         weight,
		inventory:      inventory,
		startInventory: inventory,
		startCost:      basis,
		price:          price,
	}

	// sanity
	if this.price.Sign() < 0 {
		log.Panicf("Calculated new lot (%q) price %s = %s / %s", name, this.price, this.startCost, this.startInventory)
	}
	return this
}

func (this *Lot) Sell(delta Amount) (actual, basis Amount) {
	// sanity
	if delta.Sign() > -1 {
		log.Panicf("lot.Sell() expects negative amount, got %s", delta)
	}
	if !delta.Compatible(this.inventory) {
		log.Panic("lot.Sell() account/asset mismatch")
	}

	tmp := new(big.Rat)
	tmp.Add(this.inventory.Rat, delta.Rat) // adding negative delta
	// tmp is now (inventory - amount to sell)
	switch tmp.Sign() {
	case -1:
		// inventory does not cover delta, actual is limited to inventory amount
		actual = this.inventory
		this.inventory = this.inventory.ZeroClone() // nothing remains in inventory
	case 1:
		// inventory has more than enough, put remainder back
		this.inventory.Set(tmp)
		actual = delta.NegClone()
	case 0:
		// exact amount, actual is full delta, set inventory to zero
		actual = delta.NegClone()
		this.inventory = this.inventory.ZeroClone() // nothing remains
	}

	// calculate basis that corresponds to inventory consumed
	basis = this.startCost.ZeroClone()
	basis.Mul(this.price, actual.Rat)
	basis.Neg(basis.Rat) // convention: amount sold is positive, basis is negative

	// sanity
	if actual.Sign() < 1 {
		log.Panic("lot.Sell() calculated:", actual)
	}
	if basis.Sign() > 0 { // Note that 0 basis is allowed (i.e. BCH from hard fork)
		log.Panic("lot.Sell() basis: ", basis, " from price ", this.price)
	}

	return actual, basis
}

type order string

const (
	FIFO order = "fifo" // first in, first out
	LIFO order = "lifo" // last in, first out
)

type LotQueue struct {
	lot   []Lot
	order order
}

func (this LotQueue) Len() int      { return len(this.lot) }
func (this LotQueue) Swap(i, j int) { this.lot[i], this.lot[j] = this.lot[j], this.lot[i] }
func (this LotQueue) Less(i, j int) bool {
	// we sell from the tail of slice
	switch this.order {
	case FIFO:
		// earliest lot comes last in slice
		// treat equal as later, respecting order of transactions in source
		return this.lot[i].date.After(this.lot[j].date) || (this.lot[i].date.Equal(this.lot[j].date) && this.lot[i].weight > this.lot[j].weight)
	case LIFO:
		return this.lot[i].date.Before(this.lot[j].date) || (this.lot[i].date.Equal(this.lot[j].date) && this.lot[i].weight < this.lot[j].weight)
	}
	log.Panicf("unexpected lot order (%q)", this.order)
	return false
}

func (this *LotQueue) Buy(lot Lot) {
	this.sanity(lot.inventory)
	// TODO(dnc): perhaps we can be more efficient than calling sort
	// each time, given we are already ordered.
	this.lot = append(this.lot, lot)
	sort.Sort(this)
}

// Sell consumes inventory and basis from lots.
func (this *LotQueue) Sell(delta Amount) (lot []Lot, inventory, basis []Amount, err error) {
	this.sanity(delta)
	command.V(1).Infof("LotQueue.Sell() %s from queue of %d lots", delta.String(), this.Len()) // troubleshoot

	remaining := delta.Clone()

	var l Lot
	for remaining.Sign() != 0 {

		if this.Len() == 0 {
			// We haven't consumed original delta, but the queue is empty.
			err = fmt.Errorf("failed to sell %s (of %s), no remaining inventory", remaining.String(), delta.String())
			return
		}

		// pop from end of slice
		l, this.lot = this.lot[len(this.lot)-1], this.lot[:len(this.lot)-1]

		sold, soldBasis := l.Sell(remaining)

		// sanity
		if sold.Sign() == -1 || soldBasis.Sign() == 1 { // basis may be zero
			log.Panicf("insane sale: sold %s, basis %s", sold, soldBasis)
		}

		command.V(1).Infof("Sold %s (%s basis) from lot %s", sold, soldBasis, l.name)

		lot = append(lot, l)
		inventory = append(inventory, sold)
		basis = append(basis, soldBasis)
		// note that remaining is negative, sold is positive
		remaining.Add(remaining.Rat, sold.Rat)

		if remaining.Sign() > -1 {
			// entire amount has been consumed from inventory
			if remaining.Sign() != 0 { // sanity
				log.Panic("lotFIFO.Sell() remaining:", remaining) // should never be reached
			}
			if l.inventory.Sign() > 0 {
				// append unsold inventory back to queue
				this.lot = append(this.lot, l)
			}
		}
	}

	command.V(1).Infof("LotQueue.Sell() sold %s, %d lots remain", delta.String(), this.Len()) // troubleshoot

	return lot, inventory, basis, err
}

func (this LotQueue) sanity(delta Amount) {
	if delta.Sign() == 0 {
		log.Panic("attempt to buy/sell zero amount")
	}
	if this.Len() == 0 {
		if delta.Sign() < 0 {
			log.Panicf("attempt to sell (%#v %s) from empty inventory (%#v)", delta, delta.Asset, this)
		} else {
			return
		}
	}
	// sanity
	if delta.Asset != this.lot[0].inventory.Asset {
		log.Panicf("currency mismatch: want %q, got %q", delta.Asset, this.lot[0].inventory.Asset)
	}
}
