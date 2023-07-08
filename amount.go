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
	"math/big"
	"strings"
)

// Assets are currencies, i.e. "BTC" or "ETH".
type Asset string

const AssetUnknown Asset = "" // for unbalanced splits

// Like ledger-cli, we observe the decimal places found in the source
// data, and later round to that precision.
var decimalPlaces = make(map[Asset]int)

func precision(asset Asset) int {
	p, ok := decimalPlaces[asset]
	if !ok {
		p = 6 // ledger-cli defaults to 6
	}
	return p
}

type Amount struct {
	Asset
	// we use rational numbers, because so does ledger-cli (https://www.ledger-cli.org/3.0/doc/ledger3.html#Integer-Amounts)
	*big.Rat
}

func NewAmount(asset Asset, amount big.Rat) Amount {
	return Amount{asset, &amount}
}

// We require "<amount> <asset>", i.e. "100 USD" - unlike ledger-cli
// which is supports other formats as well.
func parseAmount(str string) (this Amount, err error) {
	this.Rat = new(big.Rat)
	spacePart := strings.Split(strings.TrimSpace(str), " ")
	if len(spacePart) < 2 {
		err = fmt.Errorf("failed to parse amount (%q), expected amount and asset name", str)
		return
	}
	this.Asset = Asset(spacePart[1])

	// ledger supports math i.e. "(1 USD + 2 USD)", but we require a simple number i.e. "3 USD"
	_, ok := this.Rat.SetString(spacePart[0])
	if !ok {
		err = fmt.Errorf("failed to parse amount (%q)", str)
		return
	}
	decimalPart := strings.Split(spacePart[0], ".")
	if len(decimalPart) > 1 {
		if len(decimalPart[1]) > precision(this.Asset) {
			decimalPlaces[this.Asset] = len(decimalPart[1])
		}
	}
	return
}

// TODO(dnc): clone methods should probably return *Amount

func (this Amount) ZeroClone() Amount {
	return Amount{
		Asset: this.Asset,
		Rat:   new(big.Rat),
	}
}

func (this Amount) NegClone() Amount {
	clone := this.ZeroClone()
	clone.Set(this.Rat).Neg(this.Rat)
	return clone
}

func (this Amount) AbsClone() Amount {
	clone := this.ZeroClone()
	clone.Set(this.Rat).Abs(this.Rat)
	return clone
}

func (this Amount) Clone() Amount {
	clone := this.ZeroClone()
	clone.Set(this.Rat)
	return clone
}

func (this Amount) Compatible(x Amount) bool {
	return this.Asset == x.Asset
}

func (this Amount) FloatString() string {
	f := this.Rat.FloatString(precision(this.Asset))
	return f
}

func (this Amount) String() string {
	f := this.FloatString()

	parts := strings.Split(f, ".")
	if len(parts) > 1 {
		parts[1] = strings.TrimRight(parts[1], "0") // omit trailing 0 after decimal
		if parts[1] == "" {
			parts = parts[0:1] // omit decimal place
		}
	}
	return fmt.Sprintf("%s %s", strings.Join(parts, "."), this.Asset)
}
