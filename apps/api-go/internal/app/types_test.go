// VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
package app

import (
	"bytes"
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func goodProperty() Property {
	return Property{Address: "Example address", City: "Denver", State: "CO", ZIP: "80204", Bedrooms: 3, Bathrooms: 2, Sqft: 1500, Structure: "detached", Condition: "good", GeocodeStatus: "unverified"}
}
func TestPropertyValidation(t *testing.T) {
	p := goodProperty()
	if p.Validate() != nil {
		t.Fatal("valid property rejected")
	}
	p.Bedrooms = 7
	if p.Validate() == nil {
		t.Fatal("invalid bedrooms accepted")
	}
}
func TestNonfinite(t *testing.T) {
	p := goodProperty()
	lat, lon := 39.0, math.NaN()
	p.Latitude = &lat
	p.Longitude = &lon
	if p.Validate() == nil {
		t.Fatal("NaN longitude")
	}
	p = goodProperty()
	p.Bathrooms = math.NaN()
	if p.Validate() == nil {
		t.Fatal("NaN bath")
	}
}
func TestCoordinatePair(t *testing.T) {
	p := goodProperty()
	lat := 39.0
	p.Latitude = &lat
	if p.Validate() == nil {
		t.Fatal("coordinate pair required")
	}
}
func TestSyntheticLabel(t *testing.T) {
	p := goodProperty()
	p.GeocodeStatus = "synthetic"
	if p.Validate() == nil {
		t.Fatal("unlabeled synthetic")
	}
}
func TestUtilitiesExclusive(t *testing.T) {
	for _, x := range [][]string{{"heat_gas", "heat_electric"}, {"trash", "trash"}, {"unknown"}} {
		if (Utilities{true, x}).Validate() == nil {
			t.Fatal(x)
		}
	}
}
func TestExplicitZeroUtility(t *testing.T) {
	if (Utilities{true, []string{}}).Validate() != nil {
		t.Fatal("all owner paid is valid")
	}
}
func TestPickSpecific(t *testing.T) {
	rows := []Schedule{{Kind: "payment_standard", Bedrooms: 3, Structure: "any", Code: "none", Geo: "*", Cents: 10}, {Kind: "payment_standard", Bedrooms: 3, Structure: "any", Code: "none", Geo: "80204", Cents: 20}}
	v, e := pick(rows, "payment_standard", 3, "any", "none", "80204")
	if e != nil || v.Cents != 20 {
		t.Fatal(v, e)
	}
}
func TestPickConflict(t *testing.T) {
	r := Schedule{Kind: "utility", Bedrooms: 3, Structure: "any", Code: "trash", Geo: "*"}
	if _, e := pick([]Schedule{r, r}, "utility", 3, "any", "trash", "80204"); e == nil {
		t.Fatal("conflict not rejected")
	}
}
func TestPickMissing(t *testing.T) {
	v, e := pick(nil, "utility", 3, "any", "trash", "80204")
	if e != nil || v != nil {
		t.Fatal("missing fabricated")
	}
}
func TestStrictJSON(t *testing.T) {
	for _, raw := range []string{`{"x":1}`, `{} {}`} {
		r := httptest.NewRequest("POST", "/", bytes.NewBufferString(raw))
		var p Property
		if decode(r, &p) == nil {
			t.Fatal("unrecognized JSON accepted")
		}
	}
}
func TestAnalysisDates(t *testing.T) {
	r := AnalysisRequest{PropertyID: newID(), EffectiveOn: "2026-09-14", ComparableKind: "asking"}
	if r.Validate() != nil {
		t.Fatal("valid dates")
	}
	r.KnowledgeAt = time.Now().Add(time.Hour).Format(time.RFC3339)
	if r.Validate() == nil {
		t.Fatal("future knowledge")
	}
}
func TestIDs(t *testing.T) {
	a, b := newID(), newID()
	if a == b || !uuidPattern.MatchString(a) {
		t.Fatal("invalid IDs")
	}
}
func TestURL(t *testing.T) {
	if !validURL("https://example.invalid/source") || validURL("javascript:alert(1)") || validURL("https://user:pass@example.com") {
		t.Fatal("URL policy")
	}
}
func TestRateLimit(t *testing.T) {
	s := New(nil, Services{}, true, "")
	for i := 0; i < 120; i++ {
		if s.limited("ip") {
			t.Fatal("too early")
		}
	}
	if !s.limited("ip") {
		t.Fatal("limit missing")
	}
}
func TestServiceTokenAndResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Token") != "test-token" {
			t.Error("token missing")
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	s := Services{Client: srv.Client(), Token: "test-token"}
	if _, e := s.Call(context.Background(), srv.URL, map[string]int{"a": 1}); e != nil {
		t.Fatal(e)
	}
}
func TestServiceUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer srv.Close()
	s := Services{Client: srv.Client()}
	if _, e := s.Call(context.Background(), srv.URL, nil); e == nil {
		t.Fatal("503 ignored")
	}
}
