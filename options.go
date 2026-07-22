package repometa

// Option configures a [Scan] call. Options are applied in order; when
// two options set the same field, the later one wins.
type Option func(*options)

type options struct {
	maxDepth    int
	maxDirs     int
	maxFileSize int64
}

// Bounded-traversal caps — every recursive walk on user-supplied input
// must reference a named constant so termination is provable and the
// bound is grep-discoverable.
const (
	defaultMaxDepth    = 20
	defaultMaxDirs     = 50_000
	defaultMaxFileSize = 4 << 20 // 4 MiB per manifest file parsed
)

func defaultOptions() options {
	return options{
		maxDepth:    defaultMaxDepth,
		maxDirs:     defaultMaxDirs,
		maxFileSize: defaultMaxFileSize,
	}
}

// WithMaxDepth caps directory recursion depth. The scan root is depth 0.
// A value of 0 or less is treated as "no descent below root".
func WithMaxDepth(n int) Option { return func(o *options) { o.maxDepth = n } }

// WithMaxDirs caps the total number of directories visited during the
// scan. The walk aborts silently when this cap is hit;
// [ScanStats.DirCapHits] on the returned Manifest reports how many times
// the cap fired so callers can decide whether to widen the scan.
func WithMaxDirs(n int) Option { return func(o *options) { o.maxDirs = n } }

// WithMaxFileSize caps how many bytes any single manifest file may be
// read into memory. Files above this cap are skipped for content parsing
// but their presence is still recorded as [Evidence]. The cap protects
// against pathologically large manifest files (generated lockfiles,
// vendored artifacts) exhausting memory during a scan.
func WithMaxFileSize(n int64) Option { return func(o *options) { o.maxFileSize = n } }
