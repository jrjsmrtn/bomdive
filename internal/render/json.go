package render

import (
	"encoding/json"
	"io"
)

// JSON renders for a machine.
//
// It emits the SAME Result the text surface uses, so coverage, synthetic-root and
// no-graph caveats cannot be present for a human and missing for a pipeline. The
// audience registry records that obligation: neither audience can tell a sparse
// graph from a complete one by looking, so both surfaces must say.
func JSON(w io.Writer, r Result) error {
	if r.Entries == nil {
		r.Entries = []Entry{} // null and [] mean different things to a consumer
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
