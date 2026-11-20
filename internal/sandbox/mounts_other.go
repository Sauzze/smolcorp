//go:build !darwin

package sandbox

// OwnersMounts is none off a Mac, where no desk's profile is a Seatbelt one (FXSWEEP3P1).
func OwnersMounts() ([]Mount, error) { return nil, nil }
