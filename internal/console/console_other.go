//go:build !windows

package console

// HideIfOwned does nothing: only Windows gives a program a console window of
// its own to hide.
func HideIfOwned() {}
