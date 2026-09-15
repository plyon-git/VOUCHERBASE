// VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Services struct {
	Client              *http.Client
	Rust, Python, Token string
}

func (s Services) Call(ctx context.Context, url string, input any) (json.RawMessage, error) {
	raw, e := json.Marshal(input)
	if e != nil {
		return nil, e
	}
	r, e := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(raw))
	if e != nil {
		return nil, e
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Internal-Token", s.Token)
	res, e := s.Client.Do(r)
	if e != nil {
		return nil, errors.New("calculation_or_data_service_unavailable")
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, 8_000_001))
	if e != nil || len(b) > 8_000_000 {
		return nil, errors.New("service_response_too_large")
	}
	if res.StatusCode != 200 {
		if res.StatusCode == 422 {
			return nil, fmt.Errorf("invalid_calculation_input: %s", string(b))
		}
		return nil, errors.New("calculation_or_data_service_unavailable")
	}
	if !json.Valid(b) {
		return nil, errors.New("invalid_service_response")
	}
	return b, nil
}
func (s *Server) Evaluate(ctx context.Context, tenant string, r AnalysisRequest) (map[string]any, error) {
	tx, e := s.DB.Tx(ctx, tenant)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	p, e := property(ctx, tx, r.PropertyID)
	if e != nil {
		return nil, dbError(e)
	}
	if p.IsDemo && !s.Demo {
		return nil, errors.New("demo_records_disabled")
	}
	warnings := []string{}
	rows := []Schedule{}
	authorityOK := false
	if r.AuthorityID != "" {
		var demo bool
		var state string
		e = tx.QueryRow(ctx, "SELECT is_demo,state FROM authorities WHERE id=$1", r.AuthorityID).Scan(&demo, &state)
		if e != nil {
			return nil, errors.New("authority_not_found")
		}
		if demo != p.IsDemo {
			return nil, errors.New("demo_and_official_authority_cannot_be_mixed")
		}
		if state != "US" && state != p.State {
			return nil, errors.New("authority_state_mismatch")
		}
		authorityOK = r.AuthorityConfirmed
		rows, e = schedules(ctx, tx, r.AuthorityID, r.EffectiveOn, r.KnowledgeAt)
		if e != nil {
			return nil, e
		}
	}
	refs := []Schedule{}
	add := func(v *Schedule) *int64 {
		if v == nil {
			return nil
		}
		refs = append(refs, *v)
		n := v.Cents
		return &n
	}
	unit, e := pick(rows, "payment_standard", p.Bedrooms, "any", "none", p.ZIP)
	if e != nil {
		return nil, e
	}
	unitValue := add(unit)
	var voucherValue *int64
	ub := p.Bedrooms
	if r.VoucherBedrooms != nil {
		v, err := pick(rows, "payment_standard", *r.VoucherBedrooms, "any", "none", p.ZIP)
		if err != nil {
			return nil, err
		}
		voucherValue = add(v)
		if *r.VoucherBedrooms < ub {
			ub = *r.VoucherBedrooms
		}
	}
	var ua *int64
	if r.Utilities.Complete {
		n := int64(0)
		complete := true
		for _, code := range r.Utilities.Items {
			v, err := pick(rows, "utility", ub, p.Structure, code, p.ZIP)
			if err != nil {
				return nil, err
			}
			if v == nil {
				complete = false
				warnings = append(warnings, "MISSING_UTILITY_COMPONENT:"+code)
			} else {
				n += *add(v)
			}
		}
		if complete {
			ua = &n
		}
	}
	// Without an administering authority, a tenant-paid allowance is not established.
	if !authorityOK && len(r.Utilities.Items) > 0 {
		ua = nil
	}
	comps, e := candidates(ctx, tx, p)
	if e != nil {
		return nil, e
	}
	benchmarks := []Schedule{}
	if r.BenchmarkEntity != "" {
		hud, err := schedules(ctx, tx, "HUD", r.EffectiveOn, r.KnowledgeAt)
		if err != nil {
			return nil, err
		}
		for _, v := range hud {
			if v.Bedrooms == p.Bedrooms && ((v.Kind == "fmr" && v.Geo == r.BenchmarkEntity) || (v.Kind == "safmr" && v.Geo == p.ZIP)) {
				benchmarks = append(benchmarks, v)
			}
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	if len(benchmarks) == 0 {
		warnings = append(warnings, "HUD_BENCHMARK_NOT_LOADED")
	}
	if p.GeocodeStatus == "unverified" || p.GeocodeStatus == "user_confirmed" {
		warnings = append(warnings, "ADDRESS_LOCATION_REQUIRES_INDEPENDENT_VERIFICATION")
	}
	day, _ := time.Parse("2006-01-02", r.EffectiveOn)
	if day.After(time.Now()) {
		warnings = append(warnings, "FUTURE_EFFECTIVE_DATE_POLICY_MAY_CHANGE")
	}
	if unit != nil {
		seen, _ := time.Parse(time.RFC3339, unit.RetrievedAt)
		if time.Since(seen) > 180*24*time.Hour {
			warnings = append(warnings, "SOURCE_RECHECK_DUE")
		}
	}
	warnings = append(warnings, "PHA_SERVICE_AREA_NOT_AUTOMATICALLY_CONFIRMED")
	if p.IsDemo {
		warnings = append(warnings, "SYNTHETIC_DEMONSTRATION_ONLY")
	}
	engineRequest := map[string]any{"authority_confirmed": authorityOK, "unit_bedrooms": p.Bedrooms, "voucher_bedrooms": r.VoucherBedrooms, "unit_standard_cents": unitValue, "voucher_standard_cents": voucherValue, "utility_allowance_cents": ua, "proposed_rent_cents": r.ProposedRent, "underwriting": r.Underwriting, "scenarios": r.Scenarios}
	if r.Scenarios == nil {
		engineRequest["scenarios"] = []any{}
	}
	result, e := s.Services.Call(ctx, s.Services.Rust+"/v1/calculate", engineRequest)
	if e != nil {
		return nil, e
	}
	compReq := map[string]any{"subject": p, "candidates": comps, "effective_on": r.EffectiveOn, "knowledge_at": r.KnowledgeAt, "kind": r.ComparableKind}
	market, e := s.Services.Call(ctx, s.Services.Python+"/v1/comparables", compReq)
	if e != nil {
		market = json.RawMessage(`{"status":"service_unavailable","selected":[],"range_cents":null,"median_cents":null}`)
		warnings = append(warnings, "COMPARABLE_SERVICE_UNAVAILABLE")
	}
	return map[string]any{"property": p, "calculation": result, "market": market, "benchmarks": benchmarks, "sources": refs, "warnings": warnings, "engine_request": engineRequest, "engine_request_sha256": objectHash(engineRequest), "calculation_sha256": hash(result), "api_version": Version, "pipeline_version": "comparable-median/1.0.0", "watermark": Watermark, "utility_bedrooms": ub, "authority_confirmation_method": "user_asserted_not_independently_verified", "scope": "Property planning only. No household subsidy or PHA rent approval."}, nil
}
