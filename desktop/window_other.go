//go:build !windows

package desktop

// hasNativeWindow: only Windows has the office in a window of its own (WebView2).
// Elsewhere Show Office opens it in the browser.
const hasNativeWindow = false

// openOffice opens the office in the browser, as the Mac app's Show Office
// opens its window: with a token that works once, as for setup, which the
// deck swaps for the office's own key on this computer.
func (d *Deck) openOffice() {
	d.mu.Lock()
	token := d.access.NewOfficeToken()
	d.mu.Unlock()
	openURL(localBase() + "/office?o=" + token)
}

func (d *Deck) setAlwaysOnTop(bool) {}
