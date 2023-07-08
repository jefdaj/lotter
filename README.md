Lotter is a command-line tool that works with trade data in ledger-cli format. While ledger-cli is a fantastic calculator for double-entry accounting, its support for lots, cost basis, and gains is rather limited. This tool is meant to provide features which (to the best of my knowledge) ledger-cli does not provide on its own.

To best understand lotter, it is recommended to first be familiar with ledger-cli. Also, read background articles "Multiple Currencies with Currency Trading Accounts", and Peter Selinger's "Tutorial on Multiple Currency Accounting".

Use lotter by first entering trade information into ledger-cli. Run lotter to add "lot" information, which enables ledger-cli to calculate cost basis and gains.
Simple Example

Let's say you purchased a cryptocurrency (we'll call it ABC), when it cost 2 cents. A ledger-cli entry could look like:

2016-01-01 Bought ABC
    Assets:Crypto          100 ABC @ 0.02 USD
    Equity:Cash

Later, ABC trades at $1, and you sell some. In ledger-cli:

2017-01-01 Sell some ABC
    Assets:Crypto          -1 ABC @ 1 USD
    Assets:Exchange

The idea of lotter is to add "splits" to these ledger entries. The added information captures the cost basis when a "lot" is created, and gains (losses) when inventory from a lot is sold. After lotter, the ledger entries look like:

2016-01-01 Bought ABC
    Assets:Crypto                               100 ABC ; @ 0.02 USD
    Equity:Cash
    [Lot::2016/01/01:100ABC@0.02USD]            -100 ABC        ; :BUY: (inventory)
    [Lot::2016/01/01:100ABC@0.02USD]            2 USD           ; :BUY: (basis)

2017-01-01 Sell some ABC
    Assets:Crypto                               -1 ABC ; @ 1 USD
    Assets:Exchange
    [Lot::2016/01/01:100ABC@0.02USD]            1 ABC           ; :SELL: (inventory consumed)
    [Lot::2016/01/01:100ABC@0.02USD]            -0.02 USD       ; :SELL: (basis consumed)
    [Lot:Income:long term gain]                 -0.98 USD       ; :GAIN:LONGTERM:

If your wondering why the last line ("long term gain") shows a negative number, when the actual gain is a positive 98 cents, recall that in ledger-cli's double-entry method, income is expressed in negative numbers while expenses are positive. Similarly in lotter, lot inventory and gain are negative numbers, cost basis is positive. This follows ledger-cli's rules, and makes lotter's splits net zero.

The transactions described above are in testdata/simple.ledger. To see the effects of lotter on these transactions, compare the normal use of ledger-cli,

ledger -f testdata/simple.ledger bal

with the effects of lotter,

lotter -f testdata/simple.ledger lot | ledger -f - bal

Operation base

Usage:

lotter [-base <currency>] -f <filename> base

The base operation modifies transaction splits, converting costs and amounts into the base currency. This is intended to be a pre-processor for the lot operation, allowing trades to be accounted for in terms of the base currency, even when the trades are for other currencies.

This operation observes prices in the ledger file. When a split has a cost expressed in a currency other than base, and a price conversion to base is available on the same day as the transaction, this operation rewrites the transaction splits converting the original cost currency into the base.
Operation lot

Usage:

lotter [-base <currency>] -f <filename> lot

The lot operation adds "splits" to transactions, representing lot inventory, cost basis, and gains.

Each lot is a ledger-cli "account", named by convention with prefix "Lot", followed by the date the lot was created, and inventory and cost information. This naming convention is intended to provide unique lot names. (It could fail to do so, if multiple purchases occur on the same day, for the same amount and cost.)

lotter considers a transaction to be a purchase when it finds a split for a positive amount, with cost information associated with it. When constructing your ledger entries, use for example "100 ABC @ 0.02 USD" or "100 ABC @@ 2 USD".

Similarly, lotter considers a transaction to be a sale when the amount is negative and has a cost associated. To these transactions, lotter adds splits that "consume" inventory (and basis) acquired earlier.

To see options available, run lotter help lot.
