module src.d10.dev/lotter

go 1.15

require src.d10.dev/command v0.0.0-20201230093613-a8448f374bdf

// unfortunately need to checkout src.d10.dev/command until
// https://github.com/golang/go/issues/42323 is fixed (go1.16.0)
replace src.d10.dev/command => ../command

