package sandbox

// rowsPlaces gives a rows desk's Places what FXGITROWS2's rows take from the start: the branch the owner's checkout
// is on. (Its own branches are hire/<OwnID>, h1's.)
func rowsPlaces(pl *Places, checkoutBranch string) { pl.CheckoutBranch = checkoutBranch }
