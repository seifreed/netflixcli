package client

// The Client owns one thing: a session that can reach Netflix — the TLS
// transport, the cookie, the page bootstrap and the persisted-query map. What
// you can *ask* Netflix hangs off it as three services, grouped by the question
// they answer:
//
//	Catalog — what is on Netflix (search, genres, title detail, episodes)
//	Library — what this profile has (browse rows, My List, ratings, reminders)
//	Account — who this session is (profiles, switching, viewing history)
//
// The services depend on the client's plumbing; the plumbing knows nothing
// about them.

// Catalog answers questions about the catalogue itself. Its results depend on
// the account's region, but not on which profile is active.
type Catalog struct{ client *Client }

// Library is everything tied to the profile the session is acting as: the rows
// Netflix personalises for it, and the lists it can change.
type Library struct{ client *Client }

// Account is the session itself: who it belongs to, which profile it acts as,
// and what that profile has watched.
type Account struct{ client *Client }
