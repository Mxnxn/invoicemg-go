package store

import "sort"

// ClientAmount is one row's client key and money value, the lean projection both the invoice
// and the two receipt collections reduce to for the dues report (Helpers/ClientDues.js).
type ClientAmount struct {
	ClientID string
	Amount   float64
}

// ClientDue is one client's receivables: total billed, total received, and the outstanding
// difference, each rounded to the paisa.
type ClientDue struct {
	ClientID string
	Billed   float64
	Received float64
	Due      float64
}

// ClientBrief is the display detail joined onto a dues row.
type ClientBrief struct {
	Name  string
	Firm  string
	Phone string
}

// ClientDuesData is the dues report's raw inputs: the per-client aggregates (already computed
// and sorted) and the client directory the handler filters/labels against.
type ClientDuesData struct {
	Dues    []ClientDue
	Clients map[string]ClientBrief
}

// ComputeClientDues ports Helpers/ClientDues.computeClientDues: one row per client with any
// activity, billed from invoices and received from both receipt collections, sorted by due
// descending. First-seen order (invoices, then received, then batch) breaks a due tie, matching
// the Map insertion order V8's stable sort preserves. Deliberately NOT the top-credits formula
// (sum(totalAmount - amount)), which misses a transfer against not-yet-invoiced entries.
func ComputeClientDues(invoices, received, batch []ClientAmount) []ClientDue {
	type agg struct{ billed, received float64 }
	byClient := map[string]*agg{}
	order := []string{}
	bucket := func(key string) *agg {
		if key == "" {
			return nil
		}
		a, ok := byClient[key]
		if !ok {
			a = &agg{}
			byClient[key] = a
			order = append(order, key)
		}
		return a
	}
	for _, iv := range invoices {
		if a := bucket(iv.ClientID); a != nil {
			a.billed += iv.Amount
		}
	}
	for _, r := range received {
		if a := bucket(r.ClientID); a != nil {
			a.received += r.Amount
		}
	}
	for _, b := range batch {
		if a := bucket(b.ClientID); a != nil {
			a.received += b.Amount
		}
	}
	out := make([]ClientDue, 0, len(order))
	for _, key := range order {
		a := byClient[key]
		out = append(out, ClientDue{
			ClientID: key,
			Billed:   round2(a.billed),
			Received: round2(a.received),
			Due:      round2(a.billed - a.received),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Due > out[j].Due })
	return out
}
