// Package manifest defines stable machine-readable scanner records.
package manifest

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/dkujawski/dig-local-browser/internal/signatures"
)

type Candidate struct {
	Path            string             `json:"path"`
	Size            int64              `json:"size"`
	ModifiedAt      time.Time          `json:"modified_at"`
	CreatedAt       *time.Time         `json:"created_at,omitempty"`
	Device          uint64             `json:"device,omitempty"`
	Inode           uint64             `json:"inode,omitempty"`
	SimpleCache     bool               `json:"simple_cache"`
	CacheStructure  bool               `json:"cache_structure"`
	ImageSignatures []signatures.Match `json:"image_signatures,omitempty"`
	MatchedStrings  []string           `json:"matched_strings,omitempty"`
	Confidence      int                `json:"confidence"`
	Signals         []string           `json:"signals"`
	Errors          []string           `json:"errors"`
}

type Writer struct {
	mu  sync.Mutex
	enc *json.Encoder
}

func NewWriter(w io.Writer) *Writer { return &Writer{enc: json.NewEncoder(w)} }
func (w *Writer) Write(c Candidate) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.enc.Encode(c)
}
