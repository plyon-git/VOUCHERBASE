// VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context) (*Store, error) {
	p, e := pgxpool.New(ctx, "")
	if e != nil {
		return nil, e
	}
	if e = p.Ping(ctx); e != nil {
		p.Close()
		return nil, e
	}
	return &Store{p}, nil
}
func (s *Store) Tx(ctx context.Context, tenant string) (pgx.Tx, error) {
	t, e := s.Pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	_, e = t.Exec(ctx, "SELECT set_config('app.tenant_id',$1,true)", tenant)
	if e != nil {
		t.Rollback(ctx)
		return nil, e
	}
	return t, nil
}
func jsonRows(ctx context.Context, tx pgx.Tx, query string, args ...any) ([]json.RawMessage, error) {
	rs, e := tx.Query(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer rs.Close()
	out := []json.RawMessage{}
	for rs.Next() {
		var v []byte
		if e = rs.Scan(&v); e != nil {
			return nil, e
		}
		out = append(out, json.RawMessage(v))
	}
	return out, rs.Err()
}

const propertyColumns = `jsonb_build_object('id',p.id,'address',p.address,'city',p.city,'state',p.state,'zip',p.zip,'bedrooms',p.bedrooms,'bathrooms',p.bathrooms,'sqft',p.sqft,'structure',p.structure,'condition',p.condition,'amenities',p.amenities,'latitude',ST_Y(p.location),'longitude',ST_X(p.location),'geocode_status',p.geocode_status,'is_demo',p.is_demo)`

func property(ctx context.Context, tx pgx.Tx, id string) (Property, error) {
	var raw []byte
	p := Property{}
	e := tx.QueryRow(ctx, "SELECT "+propertyColumns+" FROM properties p WHERE p.id=$1", id).Scan(&raw)
	if e != nil {
		return p, e
	}
	e = json.Unmarshal(raw, &p)
	return p, e
}
func insertProperty(ctx context.Context, tx pgx.Tx, tenant string, p *Property) error {
	p.ID = newID()
	if p.Amenities == nil {
		p.Amenities = []string{}
	}
	_, e := tx.Exec(ctx, `INSERT INTO properties(id,tenant_id,address,city,state,zip,bedrooms,bathrooms,sqft,structure,condition,amenities,location,geocode_status,is_demo) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,CASE WHEN $13::float8 IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($14,$13),4326) END,$15,$16)`, p.ID, tenant, p.Address, p.City, p.State, p.ZIP, p.Bedrooms, p.Bathrooms, p.Sqft, p.Structure, p.Condition, p.Amenities, p.Latitude, p.Longitude, p.GeocodeStatus, p.IsDemo)
	return e
}
func audit(ctx context.Context, tx pgx.Tx, tenant, action, id string) error {
	_, e := tx.Exec(ctx, "INSERT INTO audit_events(tenant_id,action,object_id) VALUES($1,$2,$3)", tenant, action, id)
	return e
}
func schedules(ctx context.Context, tx pgx.Tx, authority, day, known string) ([]Schedule, error) {
	vals, e := jsonRows(ctx, tx, `SELECT jsonb_build_object('id',r.id,'kind',r.kind,'geo_key',r.geo_key,'bedrooms',r.bedrooms,'structure',r.structure,'utility_code',r.utility_code,'value_cents',r.value_cents,'source_id',s.id,'title',s.title,'url',s.url,'snapshot_sha256',s.snapshot_sha256,'category',s.category,'valid_from',lower(r.valid_during),'valid_until',upper(r.valid_during),'retrieved_at',s.retrieved_at) FROM schedule_rows r JOIN catalog_sources s ON r.source_id=s.id WHERE r.authority_id=$1 AND r.status='accepted' AND r.valid_during @> $2::date AND r.known_during @> $3::timestamptz ORDER BY r.id`, authority, day, known)
	if e != nil {
		return nil, e
	}
	out := []Schedule{}
	for _, v := range vals {
		var r Schedule
		if e = json.Unmarshal(v, &r); e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, nil
}
func candidates(ctx context.Context, tx pgx.Tx, p Property) ([]json.RawMessage, error) {
	if p.Latitude == nil {
		return []json.RawMessage{}, nil
	}
	return jsonRows(ctx, tx, `SELECT jsonb_build_object('id',c.id,'source_url',c.source_url,'kind',c.kind,'observed_on',c.observed_on,'recorded_at',c.recorded_at,'rent_cents',c.rent_cents,'bedrooms',c.bedrooms,'bathrooms',c.bathrooms,'sqft',c.sqft,'structure',c.structure,'condition',c.condition,'amenities',c.amenities,'latitude',ST_Y(c.location),'longitude',ST_X(c.location),'property_ref',c.property_ref,'is_demo',c.is_demo) FROM comparables c WHERE ST_DWithin(c.location::geography,ST_SetSRID(ST_MakePoint($1,$2),4326)::geography,3000) ORDER BY c.observed_on DESC,c.id LIMIT 2000`, *p.Longitude, *p.Latitude)
}
func savedByKey(ctx context.Context, tx pgx.Tx, key string) (json.RawMessage, string, error) {
	var result []byte
	var digest string
	e := tx.QueryRow(ctx, `SELECT jsonb_build_object('id',id,'property_id',property_id,'created_at',created_at,'effective_on',effective_on,'knowledge_at',knowledge_at,'result',result,'input_sha256',input_sha256,'result_sha256',result_sha256),input_sha256 FROM analyses WHERE idempotency_key=$1`, key).Scan(&result, &digest)
	return result, digest, e
}

var errConflict = errors.New("Idempotency key reused with a different request")

func (s *Store) Save(ctx context.Context, tenant string, req AnalysisRequest, inputHash, key string, result any) (json.RawMessage, error) {
	tx, e := s.Tx(ctx, tenant)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	// Serialized per tenant/key; handles concurrent identical and conflicting requests.
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", tenant+":"+key); e != nil {
		return nil, e
	}
	old, h, e := savedByKey(ctx, tx, key)
	if e == nil {
		if h != inputHash {
			return nil, errConflict
		}
		return old, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return nil, e
	}
	id := newID()
	in, _ := json.Marshal(req)
	out, e := json.Marshal(result)
	if e != nil {
		return nil, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO analyses(id,tenant_id,property_id,effective_on,knowledge_at,input,result,input_sha256,result_sha256,idempotency_key) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8,$9,$10)`, id, tenant, req.PropertyID, req.EffectiveOn, req.KnowledgeAt, string(in), string(out), inputHash, hash(out), key)
	if e != nil {
		return nil, e
	}
	if e = audit(ctx, tx, tenant, "analysis_created", id); e != nil {
		return nil, e
	}
	raw, _, e := savedByKey(ctx, tx, key)
	if e != nil {
		return nil, e
	}
	return raw, tx.Commit(ctx)
}
func (s *Store) Existing(ctx context.Context, tenant, key, inputHash string) (json.RawMessage, error) {
	tx, e := s.Tx(ctx, tenant)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	raw, h, e := savedByKey(ctx, tx, key)
	if e == nil && h != inputHash {
		return nil, errConflict
	}
	return raw, e
}
func dbError(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return fmt.Errorf("record_not_found")
	}
	return fmt.Errorf("database_operation_failed")
}
