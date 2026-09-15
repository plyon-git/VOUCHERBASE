// VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
package app

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type identity struct{ Tenant, Label, Role string }
type bucket struct {
	Count int
	Start time.Time
}
type Server struct {
	DB       *Store
	Services Services
	Demo     bool
	Origin   string
	mu       sync.Mutex
	limits   map[string]bucket
}

func New(db *Store, services Services, demo bool, origin string) *Server {
	return &Server{DB: db, Services: services, Demo: demo, Origin: origin, limits: make(map[string]bucket)}
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return errors.New("invalid_json")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("exactly_one_json_object_required")
	}
	return nil
}
func (s *Server) limited(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	b := s.limits[ip]
	if now.Sub(b.Start) > time.Minute {
		b = bucket{Start: now}
	}
	b.Count++
	if len(s.limits) > 10000 {
		for k, v := range s.limits {
			if now.Sub(v.Start) > time.Minute {
				delete(s.limits, k)
			}
		}
	}
	if len(s.limits) > 10000 {
		return true
	}
	s.limits[ip] = b
	return b.Count > 120
}
func (s *Server) Handler(static http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		w.Header().Set("X-Voucherbase-Watermark", Watermark)
		if r.URL.Path == "/healthz" {
			ctx, c := context.WithTimeout(r.Context(), 2*time.Second)
			defer c()
			if s.DB.Pool.Ping(ctx) != nil {
				fail(w, 503, "database_unavailable")
				return
			}
			writeJSON(w, 200, map[string]string{"status": "ok"})
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			if r.Method != "GET" && r.Method != "HEAD" {
				fail(w, 405, "method_not_allowed")
				return
			}
			static.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if s.limited(ip) {
			fail(w, 429, "rate_limit")
			return
		}
		if o := r.Header.Get("Origin"); o != "" && o != s.Origin {
			fail(w, 403, "origin_not_allowed")
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			fail(w, 401, "unauthorized")
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if len(token) < 32 || len(token) > 256 {
			fail(w, 401, "unauthorized")
			return
		}
		who := identity{}
		e := s.DB.Pool.QueryRow(r.Context(), "SELECT tenant_id::text,label,role FROM authenticate_key($1)", hash([]byte(token))).Scan(&who.Tenant, &who.Label, &who.Role)
		if e != nil {
			fail(w, 401, "unauthorized")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && who.Role != "owner" {
			fail(w, 403, "owner_role_required")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 2_000_000)
		ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		if e = s.route(w, r, who); e != nil {
			switch {
			case errors.Is(e, errConflict):
				fail(w, 409, e.Error())
			case errors.Is(e, pgx.ErrNoRows) || e.Error() == "record_not_found":
				fail(w, 404, "record_not_found")
			case strings.HasPrefix(e.Error(), "invalid_calculation_input"):
				fail(w, 422, e.Error())
			default:
				fail(w, 500, "operation_failed: "+safeError(e))
			}
		}
	})
}
func safeError(e error) string {
	msg := e.Error()
	for _, safe := range []string{"calculation_or_data_service_unavailable", "demo_records_disabled", "authority_not_found", "demo_and_official_authority_cannot_be_mixed", "authority_state_mismatch", "Conflicting applicable schedule rows", "database_operation_failed"} {
		if msg == safe {
			return msg
		}
	}
	return "See server operation logs; no credentials or SQL are returned"
}
func (s *Server) route(w http.ResponseWriter, r *http.Request, who identity) error {
	ctx := r.Context()
	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	if path == "/me" && r.Method == "GET" {
		writeJSON(w, 200, map[string]any{"tenant_id": who.Tenant, "label": who.Label, "role": who.Role, "demo_enabled": s.Demo, "version": Version, "watermark": Watermark})
		return nil
	}
	if path == "/geocode" && r.Method == "POST" {
		var req struct {
			Address  string `json:"address"`
			Provider string `json:"provider"`
		}
		if e := decode(r, &req); e != nil {
			fail(w, 400, e.Error())
			return nil
		}
		out, e := s.Services.Call(ctx, s.Services.Python+"/v1/geocode", req)
		if e != nil {
			return e
		}
		writeJSON(w, 200, out)
		return nil
	}
	if path == "/analyses" && r.Method == "POST" {
		var req AnalysisRequest
		if e := decode(r, &req); e != nil {
			fail(w, 400, e.Error())
			return nil
		}
		inputHash := objectHash(req)
		if e := req.Validate(); e != nil {
			fail(w, 422, e.Error())
			return nil
		}
		key := r.Header.Get("Idempotency-Key")
		if key == "" || len(key) > 150 {
			fail(w, 400, "Idempotency-Key (1..150 chars) required")
			return nil
		}
		old, e := s.DB.Existing(ctx, who.Tenant, key, inputHash)
		if e == nil {
			writeJSON(w, 200, old)
			return nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		result, e := s.Evaluate(ctx, who.Tenant, req)
		if e != nil {
			return e
		}
		out, e := s.DB.Save(ctx, who.Tenant, req, inputHash, key, result)
		if e != nil {
			return e
		}
		writeJSON(w, 201, out)
		return nil
	}
	if path == "/jobs" && r.Method == "POST" {
		return s.enqueue(w, r, who)
	}
	if path == "/imports/properties" && r.Method == "POST" {
		return s.importProperties(w, r, who)
	}
	if path == "/comparables" && r.Method == "POST" {
		return s.importComparables(w, r, who)
	}
	tx, e := s.DB.Tx(ctx, who.Tenant)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if path == "/properties" && r.Method == "POST" {
		var p Property
		if e = decode(r, &p); e != nil {
			fail(w, 400, e.Error())
			return nil
		}
		if e = p.Validate(); e != nil {
			fail(w, 422, e.Error())
			return nil
		}
		if p.IsDemo && !s.Demo {
			fail(w, 422, "demo_disabled")
			return nil
		}
		if e = insertProperty(ctx, tx, who.Tenant, &p); e != nil {
			return e
		}
		if e = audit(ctx, tx, who.Tenant, "property_created", p.ID); e != nil {
			return e
		}
		if e = tx.Commit(ctx); e != nil {
			return e
		}
		writeJSON(w, 201, p)
		return nil
	}
	if path == "/properties" && r.Method == "GET" {
		out, err := jsonRows(ctx, tx, "SELECT "+propertyColumns+" || jsonb_build_object('latest_analysis_id',a.id,'latest_result',a.result) FROM properties p LEFT JOIN LATERAL (SELECT id,result FROM analyses WHERE property_id=p.id ORDER BY created_at DESC LIMIT 1) a ON true ORDER BY p.created_at DESC LIMIT 1000")
		if err != nil {
			return err
		}
		writeJSON(w, 200, out)
		return nil
	}
	if path == "/catalog" && r.Method == "GET" {
		a, err := jsonRows(ctx, tx, `SELECT jsonb_build_object('id',id,'name',name,'state',state,'is_demo',is_demo,'boundary_verified',boundary_verified,'has_boundary',boundary IS NOT NULL) FROM authorities WHERE $1 OR NOT is_demo ORDER BY is_demo,id`, s.Demo)
		if err != nil {
			return err
		}
		sources, err := jsonRows(ctx, tx, `SELECT to_jsonb(s)-'snapshot' FROM catalog_sources s WHERE $1 OR category<>'synthetic' ORDER BY retrieved_at DESC LIMIT 100`, s.Demo)
		if err != nil {
			return err
		}
		changes, err := jsonRows(ctx, tx, `SELECT to_jsonb(c) FROM catalog_changes c JOIN catalog_sources s ON c.source_id=s.id WHERE $1 OR s.category<>'synthetic' ORDER BY c.created_at DESC LIMIT 100`, s.Demo)
		if err != nil {
			return err
		}
		writeJSON(w, 200, map[string]any{"authorities": a, "sources": sources, "changes": changes, "coverage": "DHA 2026 transcription plus explicitly synthetic fixtures. No nationwide PHA schedule coverage is claimed."})
		return nil
	}
	if path == "/authorities" && r.Method == "GET" {
		lat, e1 := strconv.ParseFloat(r.URL.Query().Get("latitude"), 64)
		lon, e2 := strconv.ParseFloat(r.URL.Query().Get("longitude"), 64)
		if e1 != nil || e2 != nil || math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			fail(w, 422, "coordinates_required")
			return nil
		}
		out, err := jsonRows(ctx, tx, `SELECT jsonb_build_object('id',id,'name',name,'is_demo',is_demo,'spatial_match',COALESCE(ST_Covers(boundary,ST_SetSRID(ST_MakePoint($1,$2),4326)),false),'boundary_verified',boundary_verified,'requires_confirmation',true) FROM authorities WHERE state=$3 AND ($4 OR NOT is_demo) ORDER BY id`, lon, lat, r.URL.Query().Get("state"), s.Demo)
		if err != nil {
			return err
		}
		writeJSON(w, 200, out)
		return nil
	}
	if strings.HasPrefix(path, "/sources/") && r.Method == "GET" {
		id := strings.TrimPrefix(path, "/sources/")
		if !uuidPattern.MatchString(id) {
			fail(w, 400, "invalid_id")
			return nil
		}
		out, err := jsonRows(ctx, tx, `SELECT to_jsonb(s) FROM catalog_sources s WHERE id=$1 AND ($2 OR category<>'synthetic')`, id, s.Demo)
		if err != nil {
			return err
		}
		if len(out) == 0 {
			return pgx.ErrNoRows
		}
		writeJSON(w, 200, out[0])
		return nil
	}
	if path == "/analyses" && r.Method == "GET" {
		out, err := jsonRows(ctx, tx, `SELECT jsonb_build_object('id',a.id,'property_id',property_id,'address',p.address,'effective_on',effective_on,'knowledge_at',knowledge_at,'created_at',a.created_at,'result_sha256',result_sha256) FROM analyses a JOIN properties p ON p.id=a.property_id ORDER BY a.created_at DESC LIMIT 200`)
		if err != nil {
			return err
		}
		writeJSON(w, 200, out)
		return nil
	}
	if strings.HasPrefix(path, "/analyses/") {
		parts := strings.Split(strings.TrimPrefix(path, "/analyses/"), "/")
		id := parts[0]
		if !uuidPattern.MatchString(id) {
			fail(w, 400, "invalid_id")
			return nil
		}
		var data []byte
		e = tx.QueryRow(ctx, `SELECT jsonb_build_object('id',id,'input',input,'result',result,'input_sha256',input_sha256,'result_sha256',result_sha256,'created_at',created_at,'effective_on',effective_on,'knowledge_at',knowledge_at) FROM analyses WHERE id=$1`, id).Scan(&data)
		if e != nil {
			return e
		}
		if len(parts) == 1 && r.Method == "GET" {
			writeJSON(w, 200, json.RawMessage(data))
			return nil
		}
		if len(parts) == 2 && parts[1] == "replay" && r.Method == "POST" {
			var saved struct {
				Result struct {
					Request json.RawMessage `json:"engine_request"`
					Hash    string          `json:"calculation_sha256"`
				} `json:"result"`
			}
			if e = json.Unmarshal(data, &saved); e != nil {
				return e
			}
			if e = tx.Commit(ctx); e != nil {
				return e
			}
			out, err := s.Services.Call(ctx, s.Services.Rust+"/v1/calculate", saved.Result.Request)
			if err != nil {
				return err
			}
			writeJSON(w, 200, map[string]any{"matched": hash(out) == saved.Result.Hash, "saved_calculation_sha256": saved.Result.Hash, "replayed_calculation_sha256": hash(out), "scope": "Exact numeric engine request replay; no fresh market data or PHA approval."})
			return nil
		}
	}
	if path == "/audit" && r.Method == "GET" {
		out, err := jsonRows(ctx, tx, `SELECT to_jsonb(e)-'tenant_id' FROM audit_events e ORDER BY occurred_at DESC LIMIT 200`)
		if err != nil {
			return err
		}
		writeJSON(w, 200, out)
		return nil
	}
	if path == "/jobs" && r.Method == "GET" {
		return s.listJobs(ctx, w, tx, "")
	}
	if strings.HasPrefix(path, "/jobs/") && r.Method == "GET" {
		id := strings.TrimPrefix(path, "/jobs/")
		if !uuidPattern.MatchString(id) {
			fail(w, 400, "invalid_id")
			return nil
		}
		return s.listJobs(ctx, w, tx, id)
	}
	fail(w, 404, "not_found")
	return nil
}
func (s *Server) importProperties(w http.ResponseWriter, r *http.Request, who identity) error {
	reader := csv.NewReader(r.Body)
	reader.FieldsPerRecord = -1
	head, e := reader.Read()
	if e != nil {
		fail(w, 400, "CSV header required")
		return nil
	}
	allowed := map[string]bool{}
	for _, h := range []string{"address", "city", "state", "zip", "bedrooms", "bathrooms", "sqft", "structure", "condition", "latitude", "longitude", "is_demo"} {
		allowed[h] = true
	}
	idx := map[string]int{}
	for i, h := range head {
		if !allowed[h] {
			fail(w, 422, "Unknown CSV column: "+h)
			return nil
		}
		if _, ok := idx[h]; ok {
			fail(w, 422, "Duplicate CSV column")
			return nil
		}
		idx[h] = i
	}
	for _, h := range []string{"address", "city", "state", "zip", "bedrooms", "bathrooms", "sqft", "structure", "condition"} {
		if _, ok := idx[h]; !ok {
			fail(w, 422, "Missing column: "+h)
			return nil
		}
	}
	props := []Property{}
	for line := 2; ; line++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(row) != len(head) {
			fail(w, 422, fmt.Sprintf("Malformed CSV row %d", line))
			return nil
		}
		if len(props) >= 1000 {
			fail(w, 413, "At most 1000 properties per import")
			return nil
		}
		get := func(k string) string {
			if j, ok := idx[k]; ok {
				return strings.TrimSpace(row[j])
			}
			return ""
		}
		br, e1 := strconv.Atoi(get("bedrooms"))
		ba, e2 := strconv.ParseFloat(get("bathrooms"), 64)
		sq, e3 := strconv.Atoi(get("sqft"))
		p := Property{Address: get("address"), City: get("city"), State: get("state"), ZIP: get("zip"), Bedrooms: br, Bathrooms: ba, Sqft: sq, Structure: get("structure"), Condition: get("condition"), GeocodeStatus: "unverified", Amenities: []string{}, IsDemo: get("is_demo") == "true"}
		if get("is_demo") != "" && get("is_demo") != "true" && get("is_demo") != "false" {
			fail(w, 422, "is_demo must be true or false")
			return nil
		}
		if e1 != nil || e2 != nil || e3 != nil {
			fail(w, 422, "Invalid property numbers")
			return nil
		}
		if get("latitude") != "" || get("longitude") != "" {
			lat, a := strconv.ParseFloat(get("latitude"), 64)
			lon, b := strconv.ParseFloat(get("longitude"), 64)
			if a != nil || b != nil {
				fail(w, 422, "Invalid coordinates")
				return nil
			}
			p.Latitude = &lat
			p.Longitude = &lon
			p.GeocodeStatus = "user_confirmed"
			if p.IsDemo {
				p.GeocodeStatus = "synthetic"
			}
		}
		if err = p.Validate(); err != nil {
			fail(w, 422, fmt.Sprintf("Row %d: %s", line, err))
			return nil
		}
		if p.IsDemo && !s.Demo {
			fail(w, 422, "demo_disabled")
			return nil
		}
		props = append(props, p)
	}
	if len(props) == 0 {
		fail(w, 422, "No property rows")
		return nil
	}
	tx, e := s.DB.Tx(r.Context(), who.Tenant)
	if e != nil {
		return e
	}
	defer tx.Rollback(r.Context())
	for i := range props {
		if e = insertProperty(r.Context(), tx, who.Tenant, &props[i]); e != nil {
			return e
		}
	}
	if e = audit(r.Context(), tx, who.Tenant, "properties_imported", strconv.Itoa(len(props))); e != nil {
		return e
	}
	if e = tx.Commit(r.Context()); e != nil {
		return e
	}
	writeJSON(w, 201, map[string]any{"imported": len(props), "properties": props})
	return nil
}

type Comparable struct {
	Property
	ExternalID  string `json:"external_id"`
	SourceURL   string `json:"source_url"`
	Kind        string `json:"kind"`
	ObservedOn  string `json:"observed_on"`
	Rent        int64  `json:"rent_cents"`
	PropertyRef string `json:"property_ref"`
}

func (s *Server) importComparables(w http.ResponseWriter, r *http.Request, who identity) error {
	var payload struct {
		Rows []Comparable `json:"rows"`
	}
	if e := decode(r, &payload); e != nil {
		fail(w, 400, e.Error())
		return nil
	}
	if len(payload.Rows) == 0 || len(payload.Rows) > 1000 {
		fail(w, 422, "1..1000 comparable rows required")
		return nil
	}
	seen := map[string]bool{}
	for _, c := range payload.Rows {
		if e := c.Property.Validate(); e != nil {
			fail(w, 422, e.Error())
			return nil
		}
		d, e := time.Parse("2006-01-02", c.ObservedOn)
		if e != nil || d.After(time.Now()) || !validURL(c.SourceURL) || len(c.SourceURL) > 2000 || !oneOf(c.Kind, "asking", "executed", "synthetic") || c.Rent <= 0 || c.Rent > 1000000000 || len(c.ExternalID) < 1 || len(c.ExternalID) > 150 || len(c.PropertyRef) < 1 || len(c.PropertyRef) > 150 || c.Latitude == nil || seen[c.ExternalID] {
			fail(w, 422, "Invalid or duplicate comparable observation")
			return nil
		}
		if c.IsDemo != (c.Kind == "synthetic") || (c.IsDemo && !s.Demo) {
			fail(w, 422, "Demo classification mismatch")
			return nil
		}
		seen[c.ExternalID] = true
	}
	tx, e := s.DB.Tx(r.Context(), who.Tenant)
	if e != nil {
		return e
	}
	defer tx.Rollback(r.Context())
	for _, c := range payload.Rows {
		a := c.Amenities
		if a == nil {
			a = []string{}
		}
		_, e = tx.Exec(r.Context(), `INSERT INTO comparables(id,tenant_id,external_id,source_url,kind,observed_on,rent_cents,bedrooms,bathrooms,sqft,structure,condition,amenities,location,property_ref,is_demo) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,ST_SetSRID(ST_MakePoint($14,$15),4326),$16,$17)`, newID(), who.Tenant, c.ExternalID, c.SourceURL, c.Kind, c.ObservedOn, c.Rent, c.Bedrooms, c.Bathrooms, c.Sqft, c.Structure, c.Condition, a, *c.Longitude, *c.Latitude, c.PropertyRef, c.IsDemo)
		if e != nil {
			fail(w, 409, "Comparable external_id already exists or constraint failed; no rows imported")
			return nil
		}
	}
	if e = audit(r.Context(), tx, who.Tenant, "comparables_imported", strconv.Itoa(len(payload.Rows))); e != nil {
		return e
	}
	if e = tx.Commit(r.Context()); e != nil {
		return e
	}
	writeJSON(w, 201, map[string]int{"imported": len(payload.Rows)})
	return nil
}
