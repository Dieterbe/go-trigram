// Package trigram is a simple trigram index
package trigram

import (
	"sort"
)

// T is a trigram
type T uint32

func (t T) String() string {
	b := [3]byte{byte(t >> 16), byte(t >> 8), byte(t)}
	return string(b[:])
}

// DocID is a document ID
type DocID uint32

// Index is a trigram index
type Index map[T][]DocID

// a special (and invalid) trigram that holds all the document IDs
const tAllDocIDs T = 0xFFFFFFFF

// Extract returns a list of all the unique trigrams in s
func Extract(s string, trigrams []T) []T {

	for i := 0; i <= len(s)-3; i++ {
		t := T(uint32(s[i])<<16 | uint32(s[i+1])<<8 | uint32(s[i+2]))
		trigrams = appendIfUnique(trigrams, t)
	}

	return trigrams
}

// ExtractAll returns a list of all the trigrams in s
func ExtractAll(s string, trigrams []T) []T {

	for i := 0; i <= len(s)-3; i++ {
		t := T(uint32(s[i])<<16 | uint32(s[i+1])<<8 | uint32(s[i+2]))
		trigrams = append(trigrams, t)
	}

	return trigrams
}

func appendIfUnique(t []T, n T) []T {
	for _, v := range t {
		if v == n {
			return t
		}
	}

	return append(t, n)
}

// NewIndex returns an index for the strings in docs
func NewIndex(docs []string) Index {

	idx := make(Index)

	var allDocIDs []DocID

	var trigrams []T

	for id, d := range docs {
		ts := ExtractAll(d, trigrams)
		docid := DocID(id)
		allDocIDs = append(allDocIDs, docid)
		for _, t := range ts {
			idxt := idx[t]
			l := len(idxt)
			if l == 0 || idxt[l-1] != docid {
				idx[t] = append(idxt, docid)
			}
		}
		trigrams = trigrams[:0]
	}

	idx[tAllDocIDs] = allDocIDs

	return idx
}

// Add adds a new string to the search index
func (idx Index) Add(s string) DocID {
	id := DocID(len(idx[tAllDocIDs]))
	idx.Insert(s, id)
	return id
}

// AddTrigrams adds a set of trigrams to the search index
func (idx Index) AddTrigrams(ts []T) DocID {
	id := DocID(len(idx[tAllDocIDs]))
	idx.InsertTrigrams(ts, id)
	return id
}

// Insert adds a string with a given document ID
func (idx Index) Insert(s string, id DocID) {
	ts := ExtractAll(s, nil)
	idx.InsertTrigrams(ts, id)
}

// InsertTrigrams adds a set of trigrams with a given document ID
func (idx Index) InsertTrigrams(ts []T, id DocID) {
	for _, t := range ts {
		idxt := idx[t]
		l := len(idxt)
		if l == 0 || idxt[l-1] != id {
			idx[t] = append(idxt, id)
		}
	}

	idx[tAllDocIDs] = append(idx[tAllDocIDs], id)
}

// Delete removes a document from the index
func (idx Index) Delete(s string, id DocID) {
	ts := ExtractAll(s, nil)
	for _, t := range ts {
		ids := idx[t]
		if ids == nil {
			continue
		}

		if len(ids) == 1 && ids[0] == id {
			delete(idx, t)
			continue
		}

		i := sort.Search(len(ids), func(i int) bool { return ids[i] >= id })

		if i != -1 && i < len(ids) && ids[i] == id {
			copy(ids[i:], ids[i+1:])
			idx[t] = ids[:len(ids)-1]
		}
	}
}

// for sorting
type docList []DocID

func (d docList) Len() int           { return len(d) }
func (d docList) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
func (d docList) Less(i, j int) bool { return d[i] < d[j] }

// Sort ensures all the document IDs are in order.
func (idx Index) Sort() {
	for _, v := range idx {
		dl := docList(v)
		if !sort.IsSorted(dl) {
			sort.Sort(dl)
		}
	}
}

// Prune removes all trigrams that are present in more than the specified percentage of the documents.
func (idx Index) Prune(pct float64) int {

	maxDocs := int(pct * float64(len(idx[tAllDocIDs])))

	var pruned int

	for k, v := range idx {
		if k != tAllDocIDs && len(v) > maxDocs {
			pruned++
			idx[k] = nil
		}
	}

	return pruned
}

// Query returns a list of document IDs that match the trigrams in the query s
func (idx Index) Query(s string) []DocID {
	ts := Extract(s, nil)
	return idx.QueryTrigrams(ts)
}

type tfList struct {
	tri  []T
	freq []int
}

func (tf tfList) Len() int { return len(tf.tri) }
func (tf tfList) Swap(i, j int) {
	tf.tri[i], tf.tri[j] = tf.tri[j], tf.tri[i]
	tf.freq[i], tf.freq[j] = tf.freq[j], tf.freq[i]
}
func (tf tfList) Less(i, j int) bool { return tf.freq[i] < tf.freq[j] }

// QueryTrigrams returns a list of document IDs that match the trigram set ts
func (idx Index) QueryTrigrams(ts []T) []DocID {

	if len(ts) == 0 {
		return idx[tAllDocIDs]
	}

	var freq []int

	for _, t := range ts {
		d, ok := idx[t]
		if !ok {
			return nil
		}
		freq = append(freq, len(d))
	}

	sort.Sort(tfList{ts, freq})

	var nonzero int
	for freq[nonzero] == 0 {
		nonzero++
	}

	ids := idx.Filter(idx[ts[nonzero]], ts[nonzero+1:])

	return ids
}

// Filter removes documents that don't contain the specified trigrams
func (idx Index) Filter(docs []DocID, ts []T) []DocID {

	result := make([]DocID, len(docs))

	for _, t := range ts {
		d, ok := idx[t]
		// unknown trigram
		if !ok {
			return nil
		}

		if d == nil {
			// the trigram was removed via Prune()
			continue
		}

		result = intersect(result[:0], docs, d)
		docs = result
	}

	return docs
}

func intersect(result, a, b []DocID) []DocID {

	var aidx, bidx int

scan:
	for aidx < len(a) && bidx < len(b) {
		if a[aidx] == b[bidx] {
			result = append(result, a[aidx])
			aidx++
			bidx++
			if aidx >= len(a) || bidx >= len(b) {
				break scan
			}
		}

		for a[aidx] < b[bidx] {
			aidx++
			if aidx >= len(a) {
				break scan
			}
		}

		for bidx < len(b) && a[aidx] > b[bidx] {
			bidx++
			if bidx >= len(b) {
				break scan
			}
		}
	}

	return result
}
