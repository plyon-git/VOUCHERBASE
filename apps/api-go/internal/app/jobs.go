// VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/http"
	"time"
)

func (s *Server) enqueue(w http.ResponseWriter, r *http.Request, who identity) error {
	var req struct {
		Items []AnalysisRequest `json:"items"`
	}
	if e := decode(r, &req); e != nil {
		fail(w, 400, e.Error())
		return nil
	}
	if len(req.Items) < 1 || len(req.Items) > 1000 {
		fail(w, 422, "1..1000 analysis items required")
		return nil
	}
	digest := objectHash(req)
	for i := range req.Items {
		if e := req.Items[i].Validate(); e != nil {
			fail(w, 422, fmt.Sprintf("Item %d: %s", i, e))
			return nil
		}
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" || len(key) > 150 {
		fail(w, 400, "Idempotency-Key required")
		return nil
	}
	ctx := r.Context()
	tx, e := s.DB.Tx(ctx, who.Tenant)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", who.Tenant+":job:"+key); e != nil {
		return e
	}
	var existing, h string
	e = tx.QueryRow(ctx, "SELECT id::text,payload_sha256 FROM jobs WHERE idempotency_key=$1", key).Scan(&existing, &h)
	if e == nil {
		if h != digest {
			return errConflict
		}
		writeJSON(w, 200, map[string]string{"id": existing})
		return nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	for _, item := range req.Items {
		if _, e = property(ctx, tx, item.PropertyID); e != nil {
			return dbError(e)
		}
	}
	id := newID()
	_, e = tx.Exec(ctx, "INSERT INTO jobs(id,tenant_id,idempotency_key,payload_sha256) VALUES($1,$2,$3,$4)", id, who.Tenant, key, digest)
	if e != nil {
		return e
	}
	for i, item := range req.Items {
		raw, _ := json.Marshal(item)
		_, e = tx.Exec(ctx, "INSERT INTO job_items(id,tenant_id,job_id,item_index,request) VALUES($1,$2,$3,$4,$5::jsonb)", newID(), who.Tenant, id, i, string(raw))
		if e != nil {
			return e
		}
	}
	if e = audit(ctx, tx, who.Tenant, "batch_queued", id); e != nil {
		return e
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	writeJSON(w, 202, map[string]any{"id": id, "items": len(req.Items)})
	return nil
}
func (s *Server) listJobs(ctx context.Context, w http.ResponseWriter, tx pgx.Tx, id string) error {
	out, e := jsonRows(ctx, tx, `SELECT jsonb_build_object('id',j.id,'created_at',j.created_at,'total',count(i.id),'complete',count(*) FILTER(WHERE i.status='complete'),'failed',count(*) FILTER(WHERE i.status='failed'),'running',count(*) FILTER(WHERE i.status='running'),'queued',count(*) FILTER(WHERE i.status='queued')) FROM jobs j JOIN job_items i ON i.job_id=j.id WHERE ($1='' OR j.id::text=$1) GROUP BY j.id ORDER BY j.created_at DESC LIMIT 100`, id)
	if e != nil {
		return e
	}
	if id != "" {
		if len(out) == 0 {
			return pgx.ErrNoRows
		}
		items, err := jsonRows(ctx, tx, "SELECT jsonb_build_object('index',item_index,'status',status,'attempts',attempts,'analysis_id',analysis_id,'error',error) FROM job_items WHERE job_id=$1 ORDER BY item_index", id)
		if err != nil {
			return err
		}
		writeJSON(w, 200, map[string]any{"job": out[0], "items": items})
	} else {
		writeJSON(w, 200, out)
	}
	return nil
}
func (s *Server) Work(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rs, e := s.DB.Pool.Query(ctx, "SELECT id::text FROM work_tenants()")
			if e != nil {
				continue
			}
			ids := []string{}
			for rs.Next() {
				var id string
				if rs.Scan(&id) == nil {
					ids = append(ids, id)
				}
			}
			rs.Close()
			for _, tenant := range ids {
				if ctx.Err() != nil {
					return
				}
				s.workOne(ctx, tenant)
			}
		}
	}
}
func (s *Server) workOne(parent context.Context, tenant string) {
	ctx, cancel := context.WithTimeout(parent, 40*time.Second)
	defer cancel()
	tx, e := s.DB.Tx(ctx, tenant)
	if e != nil {
		return
	}
	defer tx.Rollback(ctx)
	_, _ = tx.Exec(ctx, "UPDATE job_items SET status='failed',error='lease_expired_after_max_attempts' WHERE status='running' AND lease_until<now() AND attempts>=3")
	lease := newID()
	var id string
	var raw []byte
	var attempts int
	e = tx.QueryRow(ctx, `WITH candidate AS (SELECT id FROM job_items WHERE (status='queued' OR (status='running' AND lease_until<now())) AND attempts<3 ORDER BY updated_at FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE job_items i SET status='running',attempts=attempts+1,lease_until=now()+interval '2 minutes',lease_token=$1,updated_at=now() FROM candidate c WHERE i.id=c.id RETURNING i.id::text,i.request,i.attempts`, lease).Scan(&id, &raw, &attempts)
	if errors.Is(e, pgx.ErrNoRows) {
		_ = tx.Commit(ctx)
		return
	}
	if e != nil {
		return
	}
	if tx.Commit(ctx) != nil {
		return
	}
	var req AnalysisRequest
	err := json.Unmarshal(raw, &req)
	var saved json.RawMessage
	if err == nil {
		result, e := s.Evaluate(ctx, tenant, req)
		err = e
		if err == nil {
			saved, err = s.DB.Save(ctx, tenant, req, objectHash(req), "batch:"+id, result)
		}
	}
	// A separate timeout lets failed/cancelled requests relinquish their leases safely.
	finish, stop := context.WithTimeout(parent, 5*time.Second)
	defer stop()
	end, e := s.DB.Tx(finish, tenant)
	if e != nil {
		return
	}
	defer end.Rollback(finish)
	if err == nil {
		var result struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(saved, &result) != nil {
			return
		}
		_, e = end.Exec(finish, "UPDATE job_items SET status='complete',analysis_id=$1,lease_until=NULL,error=NULL,updated_at=now() WHERE id=$2 AND lease_token=$3", result.ID, id, lease)
	} else {
		status := "queued"
		if attempts >= 3 {
			status = "failed"
		}
		_, e = end.Exec(finish, "UPDATE job_items SET status=$1,error=$2,lease_until=NULL,updated_at=now() WHERE id=$3 AND lease_token=$4", status, safeError(err), id, lease)
	}
	if e == nil {
		_ = end.Commit(finish)
	}
}
