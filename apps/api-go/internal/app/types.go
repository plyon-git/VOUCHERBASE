// VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const Watermark = "PL-VOUCHERBASE-20260914"
const Version = "api-go/1.0.0"

var uuidPattern = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
var zipPattern = regexp.MustCompile(`^\d{5}$`)
var statePattern = regexp.MustCompile(`^[A-Z]{2}$`)

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func hash(v []byte) string    { s := sha256.Sum256(v); return hex.EncodeToString(s[:]) }
func objectHash(v any) string { b, _ := json.Marshal(v); return hash(b) }
func oneOf(v string, choices ...string) bool {
	for _, s := range choices {
		if v == s {
			return true
		}
	}
	return false
}
func validURL(s string) bool {
	u, e := url.Parse(s)
	return e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}

type Property struct {
	ID            string   `json:"id"`
	Address       string   `json:"address"`
	City          string   `json:"city"`
	State         string   `json:"state"`
	ZIP           string   `json:"zip"`
	Bedrooms      int      `json:"bedrooms"`
	Bathrooms     float64  `json:"bathrooms"`
	Sqft          int      `json:"sqft"`
	Structure     string   `json:"structure"`
	Condition     string   `json:"condition"`
	Amenities     []string `json:"amenities"`
	Latitude      *float64 `json:"latitude"`
	Longitude     *float64 `json:"longitude"`
	GeocodeStatus string   `json:"geocode_status"`
	IsDemo        bool     `json:"is_demo"`
}

func (p Property) Validate() error {
	if len(strings.TrimSpace(p.Address)) < 3 || len(p.Address) > 500 || len(p.City) < 1 || len(p.City) > 100 || !statePattern.MatchString(p.State) || !zipPattern.MatchString(p.ZIP) {
		return errors.New("Address, city, two-letter state and five-digit ZIP are required")
	}
	if p.Bedrooms < 0 || p.Bedrooms > 6 || math.IsNaN(p.Bathrooms) || math.IsInf(p.Bathrooms, 0) || p.Bathrooms <= 0 || p.Bathrooms > 20 || p.Sqft < 100 || p.Sqft > 30000 {
		return errors.New("Property dimensions outside supported range")
	}
	if !oneOf(p.Structure, "detached", "attached", "high_rise") || !oneOf(p.Condition, "poor", "fair", "average", "good", "renovated") {
		return errors.New("Unsupported structure or condition")
	}
	if (p.Latitude == nil) != (p.Longitude == nil) {
		return errors.New("Both coordinates are required")
	}
	if p.Latitude != nil && (math.IsNaN(*p.Latitude) || math.IsInf(*p.Latitude, 0) || math.Abs(*p.Latitude) > 90 || math.IsNaN(*p.Longitude) || math.IsInf(*p.Longitude, 0) || math.Abs(*p.Longitude) > 180) {
		return errors.New("Invalid coordinates")
	}
	if len(p.Amenities) > 20 {
		return errors.New("At most 20 amenities")
	}
	for _, v := range p.Amenities {
		if len(v) > 50 {
			return errors.New("Amenity too long")
		}
	}
	if !oneOf(p.GeocodeStatus, "unverified", "user_confirmed", "census_interpolated", "synthetic") {
		return errors.New("Geocode provenance is required")
	}
	if p.GeocodeStatus == "synthetic" && !p.IsDemo {
		return errors.New("Synthetic geocoding requires demo property")
	}
	if p.Latitude == nil && p.GeocodeStatus != "unverified" {
		return errors.New("Unlocated property cannot claim a geocode")
	}
	return nil
}

type Utilities struct {
	Complete bool     `json:"complete"`
	Items    []string `json:"items"`
}

var utilityGroup = map[string]string{"heat_gas": "heat", "heat_electric": "heat", "heat_pump": "heat", "hot_water_gas": "water_heat", "hot_water_electric": "water_heat", "range_gas": "range", "range_electric": "range", "other_electric": "other", "water_sewer": "water", "water_septic": "water", "trash": "trash", "tenant_range": "appliance_range", "tenant_refrigerator": "appliance_fridge"}

func (u Utilities) Validate() error {
	seen := map[string]bool{}
	for _, c := range u.Items {
		g, ok := utilityGroup[c]
		if !ok || seen[g] {
			return errors.New("Unknown, duplicate or conflicting utility selections")
		}
		seen[g] = true
	}
	return nil
}

type AnalysisRequest struct {
	PropertyID         string            `json:"property_id"`
	EffectiveOn        string            `json:"effective_on"`
	KnowledgeAt        string            `json:"knowledge_at"`
	AuthorityID        string            `json:"authority_id"`
	AuthorityConfirmed bool              `json:"authority_confirmed"`
	VoucherBedrooms    *int              `json:"voucher_bedrooms"`
	Utilities          Utilities         `json:"utilities"`
	ProposedRent       *int64            `json:"proposed_rent_cents"`
	Underwriting       json.RawMessage   `json:"underwriting"`
	Scenarios          []json.RawMessage `json:"scenarios"`
	ComparableKind     string            `json:"comparable_kind"`
	BenchmarkEntity    string            `json:"benchmark_entity"`
}

func (r *AnalysisRequest) Validate() error {
	if !uuidPattern.MatchString(r.PropertyID) {
		return errors.New("Invalid property_id")
	}
	d, e := time.Parse("2006-01-02", r.EffectiveOn)
	if e != nil || d.Year() < 2000 || d.Year() > 2100 {
		return errors.New("Invalid effective_on date")
	}
	if r.KnowledgeAt == "" {
		r.KnowledgeAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	t, e := time.Parse(time.RFC3339Nano, r.KnowledgeAt)
	if e != nil || t.After(time.Now().Add(time.Minute)) {
		return errors.New("Knowledge timestamp must not be in the future")
	}
	if len(r.AuthorityID) > 32 || len(r.BenchmarkEntity) > 30 {
		return errors.New("Invalid authority or benchmark ID")
	}
	if r.VoucherBedrooms != nil && (*r.VoucherBedrooms < 0 || *r.VoucherBedrooms > 6) {
		return errors.New("Voucher bedrooms outside supported range")
	}
	if r.ProposedRent != nil && (*r.ProposedRent < 0 || *r.ProposedRent > 1000000000000) {
		return errors.New("Invalid proposed rent")
	}
	if !oneOf(r.ComparableKind, "executed", "asking", "synthetic") {
		return errors.New("Choose executed, asking or synthetic comparable observations")
	}
	if len(r.Scenarios) > 32 {
		return errors.New("At most 32 scenarios")
	}
	return r.Utilities.Validate()
}

type Schedule struct {
	ID             string  `json:"id"`
	Kind           string  `json:"kind"`
	Geo            string  `json:"geo_key"`
	Bedrooms       int     `json:"bedrooms"`
	Structure      string  `json:"structure"`
	Code           string  `json:"utility_code"`
	Cents          int64   `json:"value_cents"`
	SourceID       string  `json:"source_id"`
	Title          string  `json:"title"`
	URL            string  `json:"url"`
	Snapshot       string  `json:"snapshot_sha256"`
	Category       string  `json:"category"`
	EffectiveFrom  string  `json:"valid_from"`
	EffectiveUntil *string `json:"valid_until"`
	RetrievedAt    string  `json:"retrieved_at"`
}

// Prefer an exact ZIP over a general schedule, but never arbitrarily choose a conflict.
func pick(rows []Schedule, kind string, br int, structure, code, zip string) (*Schedule, error) {
	var found *Schedule
	rank := -1
	for i := range rows {
		s := &rows[i]
		if s.Kind != kind || s.Bedrooms != br || s.Code != code || !(s.Structure == structure || s.Structure == "any") || !(s.Geo == zip || s.Geo == "*") {
			continue
		}
		n := 0
		if s.Geo == zip {
			n += 2
		}
		if s.Structure == structure {
			n++
		}
		if n > rank {
			found = s
			rank = n
		} else if n == rank {
			return nil, errors.New("Conflicting applicable schedule rows")
		}
	}
	return found, nil
}
