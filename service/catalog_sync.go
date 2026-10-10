package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/internal/catalogtransport"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
)

var ErrCatalogSyncConfiguration = errors.New("catalog sync configuration unavailable")
var ErrCatalogSyncReaderDenied = errors.New("catalog sync reader denied")

// CatalogSyncStatus is configuration readiness, not database/runtime eligibility.
// It contains no credential, verifier or private configuration.
type CatalogSyncStatus struct {
	Role            string `json:"role"`
	SourceID        string `json:"source_id,omitempty"`
	TargetID        string `json:"target_id,omitempty"`
	SourceReady     bool   `json:"source_ready"`
	TargetReady     bool   `json:"target_ready"`
	ManagementReady bool   `json:"management_ready"`
	Error           string `json:"error,omitempty"`
}
type catalogReader struct {
	digest [32]byte
	active bool
	bucket catalogReaderBucket
}
type catalogReaderBucket struct {
	started  time.Time
	attempts int
}

// CatalogSyncRuntime is immutable configuration with bounded reader counters.
// Construct once per process; rotation uses a controlled process restart.
type CatalogSyncRuntime struct {
	status  CatalogSyncStatus
	origin  string
	token   string
	mu      sync.Mutex
	readers map[string]*catalogReader
	unknown catalogReaderBucket
}

// NewCatalogSyncRuntime accepts a controlled environment lookup, never HTTP
// input or general options. Invalid configuration returns an unusable runtime.
func NewCatalogSyncRuntime(getenv func(string) string) (*CatalogSyncRuntime, error) {
	r := &CatalogSyncRuntime{status: CatalogSyncStatus{Role: "disabled"}}
	bad := func() (*CatalogSyncRuntime, error) {
		return &CatalogSyncRuntime{status: CatalogSyncStatus{Role: "disabled", Error: "configuration_invalid"}}, ErrCatalogSyncConfiguration
	}
	if getenv == nil {
		return bad()
	}
	role, source, target := getenv("CATALOG_SYNC_ROLE"), getenv("CATALOG_SYNC_SOURCE_ID"), getenv("CATALOG_SYNC_TARGET_ID")
	if role == "" {
		role = "disabled"
	}
	if role != "disabled" && role != "source" && role != "target" {
		return bad()
	}
	if source != "" && !catalogSyncID(source) || target != "" && !catalogSyncID(target) {
		return bad()
	}
	origin := getenv("CATALOG_SYNC_EXTERNAL_ORIGIN")
	if origin != "" {
		var err error
		origin, err = normalizeCatalogOrigin(origin)
		if err != nil {
			return bad()
		}
	}
	token, readers, single := getenv("CATALOG_SYNC_TOKEN"), getenv("CATALOG_SYNC_READERS_JSON"), getenv("CATALOG_SYNC_SINGLE_INSTANCE")
	if single != "" && single != "true" {
		return bad()
	}
	r.status.Role, r.status.SourceID, r.status.TargetID, r.origin = role, source, target, origin
	if role == "disabled" {
		if token != "" || readers != "" {
			return bad()
		}
		return r, nil
	}
	if single != "true" || source == "" {
		return bad()
	}
	if role == "target" {
		if target == "" || target == source || readers != "" {
			return bad()
		}
		if _, ok := catalogReaderTokenID(token); !ok {
			return bad()
		}
		r.token = token
		r.status.TargetReady = true
		r.status.ManagementReady = origin != ""
		if origin == "" {
			r.status.Error = "external_origin_missing"
		}
		return r, nil
	}
	if target != "" || token != "" || len(readers) == 0 || len(readers) > 32*1024 {
		return bad()
	}
	if common.ValidateJsonNoDuplicateKeys([]byte(readers)) != nil {
		return bad()
	}
	var configured map[string]common.RawMessage
	if common.UnmarshalJsonStr(readers, &configured) != nil || len(configured) == 0 || len(configured) > 64 {
		return bad()
	}
	r.readers = make(map[string]*catalogReader, len(configured))
	for id, raw := range configured {
		if !catalogSyncID(id) {
			return bad()
		}
		var fields map[string]string
		if common.Unmarshal(raw, &fields) != nil || len(fields) != 2 {
			return bad()
		}
		verifier, status := fields["verifier"], fields["status"]
		if status != "active" && status != "revoked" || len(verifier) != 71 || !strings.HasPrefix(verifier, "sha256:") || strings.ToLower(verifier) != verifier {
			return bad()
		}
		digest, err := hex.DecodeString(verifier[7:])
		if err != nil {
			return bad()
		}
		reader := &catalogReader{active: status == "active"}
		copy(reader.digest[:], digest)
		r.readers[id] = reader
		r.status.SourceReady = r.status.SourceReady || reader.active
	}
	if !r.status.SourceReady {
		r.status.Error = "no_active_readers"
	}
	return r, nil
}

func catalogSyncID(id string) bool {
	if len(id) < 1 || len(id) > 64 {
		return false
	}
	for i, c := range []byte(id) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || i > 0 && (c == '_' || c == '-') {
			continue
		}
		return false
	}
	return true
}
func catalogReaderTokenID(token string) (string, bool) {
	if len(token) > 108 {
		return "", false
	}
	id, secret, ok := strings.Cut(token, ".")
	if !ok || !catalogSyncID(id) || len(secret) != 43 {
		return id, false
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(secret)
	return id, err == nil && len(decoded) == 32 && base64.RawURLEncoding.EncodeToString(decoded) == secret
}

func normalizeCatalogOrigin(raw string) (string, error) {
	if len(raw) > 512 || strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "?#\\") {
		return "", ErrCatalogSyncConfiguration
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" || u.Host == "" || u.Path != "" || u.RawPath != "" {
		return "", ErrCatalogSyncConfiguration
	}
	host := strings.ToLower(u.Hostname())
	// Canonical public DNS names only; no IP spellings or local hostnames.
	if net.ParseIP(host) != nil || !strings.Contains(host, ".") || len(host) > 253 {
		return "", ErrCatalogSyncConfiguration
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", ErrCatalogSyncConfiguration
		}
		for _, c := range []byte(label) {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return "", ErrCatalogSyncConfiguration
			}
		}
	}
	last := host[strings.LastIndex(host, ".")+1:]
	if last[0] < 'a' || last[0] > 'z' || last == "localhost" || last == "local" || last == "internal" {
		return "", ErrCatalogSyncConfiguration
	}
	port := u.Port()
	if strings.HasSuffix(u.Host, ":") {
		return "", ErrCatalogSyncConfiguration
	}
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
			return "", ErrCatalogSyncConfiguration
		}
		if port != "443" {
			host += ":" + port
		}
	}
	return "https://" + host, nil
}

func (r *CatalogSyncRuntime) Status() CatalogSyncStatus {
	if r == nil {
		return CatalogSyncStatus{Role: "disabled", Error: "configuration_invalid"}
	}
	return r.status
}
func (r *CatalogSyncRuntime) MarshalJSON() ([]byte, error) { return common.Marshal(r.Status()) }
func (r *CatalogSyncRuntime) String() string {
	data, _ := common.Marshal(r.Status())
	return string(data)
}
func (r *CatalogSyncRuntime) GoString() string { return r.String() }
func (r *CatalogSyncRuntime) ExternalOrigin() (string, error) {
	if r == nil || r.origin == "" {
		return "", ErrCatalogSyncConfiguration
	}
	return r.origin, nil
}

// AuthenticateReader grants no root identity, management session or proof.
// All outcomes consume attempts; unknown IDs share one bounded bucket. Fixed
// windows start on first attempt and permit 60 attempts/minute per runtime.
func (r *CatalogSyncRuntime) AuthenticateReader(token string) error {
	if r == nil {
		return ErrCatalogSyncReaderDenied
	}
	id, valid := catalogReaderTokenID(token)
	if len(token) > 108 {
		token = ""
	}
	digest := sha256.Sum256([]byte(token))
	r.mu.Lock()
	defer r.mu.Unlock()
	reader, known := r.readers[id]
	var expected [32]byte
	bucket, active := &r.unknown, false
	if known {
		expected = reader.digest
		bucket = &reader.bucket
		active = reader.active
	}
	match := subtle.ConstantTimeCompare(digest[:], expected[:]) == 1
	now := time.Now()
	if bucket.started.IsZero() || now.Sub(bucket.started) >= time.Minute {
		bucket.started = now
		bucket.attempts = 0
	}
	allowed := bucket.attempts < 60
	if allowed {
		bucket.attempts++
	}
	if r.status.SourceReady && known && active && valid && match && allowed {
		return nil
	}
	return ErrCatalogSyncReaderDenied
}

var catalogSyncEnvironment = sync.OnceValues(func() (*CatalogSyncRuntime, error) { return NewCatalogSyncRuntime(os.Getenv) })

// CurrentCatalogSyncRuntime loads protected environment configuration once.
// Invalid configuration remains failed closed until process restart.
func CurrentCatalogSyncRuntime() (*CatalogSyncRuntime, error) { return catalogSyncEnvironment() }

// FetchDevCatalog never accepts a caller-supplied source URL or HTTP client.
func FetchDevCatalog(ctx context.Context) (catalogmanifest.Snapshot, error) {
	r, err := CurrentCatalogSyncRuntime()
	if err != nil || !r.Status().TargetReady {
		return catalogmanifest.Snapshot{}, ErrCatalogSyncConfiguration
	}
	return catalogtransport.Fetch(ctx, r.token, r.status.SourceID)
}
