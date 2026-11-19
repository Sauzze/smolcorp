package desktop

import (
	"encoding/json"
	"net/http"
	"time"
)

// A floor's feed (1.26, FL1): GET /peer/events, another Mac's stream of this one's desks, as /events is a phone's.
// Of the `peer` class alone (peers_mac.go): let in by servePeer, with a peer key, over Tailscale, with the setting
// on, never as a phone or the window. The home Mac reads it to draw this one as a floor (FL2a), read-only.
//
// What it carries is built from the phone's own state, field by field from a short list (peerFeedOf), never by
// taking that state and removing what isn't allowed: so it is never more than a phone sees, and less. The ground
// floor's desks, each with who sits there and what they're doing, and this computer's clock. No buttons (what the
// desk lets you press is for FL3b, under its own version), no setting, no hiring, no music, no layout, no floors
// upstairs, nothing typed or dictated.
//
// The key is asked again while it streams, every peerStreamCheck, and the stream is cut the moment the Mac is
// removed, the setting turned off or the records reset (hangUpPeers), not at its next request (RT126M1 N2).

// peerFeedVersion is the feed's version, in every message: a home Mac that knows an older one shows it read-only,
// and one that doesn't know a newer one reads what it knows of it. 1: the desks, read-only.
const peerFeedVersion = 1

// peerStreamCheck is how often a peer's stream asks whether its key still lets it in, and pings it when it does.
var peerStreamCheck = 10 * time.Second

// peerDeskFields are what the feed tells of a desk's session, of all the phone is told: who, what they're doing,
// and since when. Not what it lets you press (keys, fire, dial, mic, dictates, phoneMic, summary, conversation,
// choice), what was last typed there (messaged), a dictation's note, a bot's last words, who hired it, a T3 desk's
// branch: none of those leave for another Mac.
var peerDeskFields = []string{
	"id", "name", "project", "state", "effort", "pending", "agent", "agentName", "activity", "tool", "model",
	"subagents", "compacting", "turnStartedAt", "errorKind", "startedAt", "role", "cloud",
}

func init() {
	registerPhoneRoute(phoneRoute{method: http.MethodGet, path: "/peer/events", access: accessPeer,
		refusal: func(d *Deck) string { return d.inWords(peerOnlyRefusal) },
		serve: func(d *Deck, w http.ResponseWriter, r *request, _ string) {
			d.peerEvents(w, r)
		}})
}

// peerEvents keeps another Mac's feed open, for as long as its key lets it in.
func (d *Deck) peerEvents(w http.ResponseWriter, r *request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	key := r.Header.Get(peerKeyHeader)
	stream := d.attachPeer(r.peerID, key)
	if stream == nil {
		// Removed, or the setting turned off, since servePeer let it in.
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	defer d.detachPeer(stream)
	keepOpen(w)
	writeEvents(w, r, flusher, stream, func() bool {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.peerLetIn(r.peerID, key)
	})
}

// attachPeer opens a stream for the Mac id, sent its first message now; nil when its key no longer lets it in.
func (d *Deck) attachPeer(id, key string) *eventStream {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.peerLetIn(id, key) {
		return nil
	}
	if d.peerStreams == nil {
		d.peerStreams = map[*eventStream]string{}
	}
	stream := newEventStream()
	d.peerStreams[stream] = id
	stream.send(peerFeedOf(d.snapshot()))
	return stream
}

func (d *Deck) detachPeer(stream *eventStream) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.peerStreams, stream)
}

// peerLetIn says whether key is still the Mac id's, with the setting on, and marks it seen when it is. With d.mu held.
func (d *Deck) peerLetIn(id, key string) bool {
	peer := d.macPeers.withKey(key)
	if !d.config.FloorsFromOtherMacs || peer == nil || peer.ID != id {
		return false
	}
	d.macPeers.markSeen(peer)
	return true
}

// hangUpPeers cuts the feed of the Mac id, or every Mac's when id is "": as it's removed, the setting turned off or
// the records reset, so nothing more is sent to it. With d.mu held.
func (d *Deck) hangUpPeers(id string) {
	for stream, peer := range d.peerStreams {
		if id == "" || peer == id {
			stream.close()
		}
	}
}

// sendPeers sends the Macs listening the feed of state, the phone's state just made (changed). With d.mu held.
func (d *Deck) sendPeers(state []byte) {
	if len(d.peerStreams) == 0 {
		return
	}
	feed := peerFeedOf(state)
	for stream := range d.peerStreams {
		stream.send(feed)
	}
}

// peerFeedOf is a feed message from the phone's state (snapshot): the version, the clock, and each desk downstairs
// with its folder's name and, of its session, peerDeskFields alone.
func peerFeedOf(state []byte) []byte {
	var phone struct {
		Keys []struct {
			Pin     any            `json:"pin"`
			Session map[string]any `json:"session"`
		} `json:"keys"`
		Now int64 `json:"now"`
	}
	_ = json.Unmarshal(state, &phone)
	desks := make([]map[string]any, len(phone.Keys))
	for i, key := range phone.Keys {
		pin, _ := key.Pin.(string)
		desk := map[string]any{"pin": orNull(pin), "session": nil}
		if key.Session != nil {
			session := map[string]any{}
			for _, field := range peerDeskFields {
				if value, ok := key.Session[field]; ok {
					session[field] = value
				}
			}
			desk["session"] = session
		}
		desks[i] = desk
	}
	feed, _ := json.Marshal(map[string]any{"version": peerFeedVersion, "now": phone.Now, "keys": desks})
	return feed
}
