// VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
package main

import (
	"context"
	bundle "github.com/plyon-git/VOUCHERBASE/apps/api-go"
	"github.com/plyon-git/VOUCHERBASE/apps/api-go/internal/app"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	token := os.Getenv("VB_INTERNAL_TOKEN")
	if len(token) < 32 {
		log.Fatal("VB_INTERNAL_TOKEN must contain at least 32 characters")
	}
	db, e := app.Open(ctx)
	if e != nil {
		log.Fatal("Database connection unavailable")
	}
	defer db.Pool.Close()
	services := app.Services{Client: &http.Client{Timeout: 30 * time.Second}, Rust: env("VB_RUST_URL", "http://localhost:8081"), Python: env("VB_PYTHON_URL", "http://localhost:8082"), Token: token}
	s := app.New(db, services, os.Getenv("VB_DEMO") == "1", env("VB_PUBLIC_ORIGIN", "http://localhost:8080"))
	assets, _ := fs.Sub(bundle.Files, "web")
	server := &http.Server{Addr: env("VB_LISTEN", ":8080"), Handler: s.Handler(http.FileServer(http.FS(assets))), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	for i := 0; i < 2; i++ {
		go s.Work(ctx)
	}
	go func() {
		<-ctx.Done()
		closeCtx, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		_ = server.Shutdown(closeCtx)
	}()
	log.Printf("VOUCHERBASE %s | Parrish Lyon | %s", app.Version, app.Watermark)
	if e = server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
}
