package northstar_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/northstar"
)

func TestGenerateDeterministic(t *testing.T) {
	a, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	b, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	ha, err := event.ContentHash(a.Event)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := event.ContentHash(b.Event)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("hashes differ: %s vs %s", ha, hb)
	}
	ca, err := event.CanonicalBytes(a.Event)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := event.CanonicalBytes(b.Event)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ca, cb) {
		t.Fatal("canonical bytes differ across generations")
	}
}

func TestGenerateMatchesGolden(t *testing.T) {
	fx, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	got, err := event.CanonicalBytes(fx.Event)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "order_line_v1.jcs"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch\ngot:  %s\nwant: %s", got, want)
	}
	hash, err := event.ContentHash(fx.Event)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := strings.TrimSpace(string(mustRead(t, filepath.Join("testdata", "order_line_v1.sha256"))))
	if hash != wantHash {
		t.Fatalf("hash = %s, want %s", hash, wantHash)
	}
	if fx.ItemID != "item-flour-001" || fx.QuantityBefore != 10 || fx.QuantityAfter != 8 {
		t.Fatalf("unexpected fixture values: %+v", fx)
	}
}

func TestGenerateUnsupportedSeed(t *testing.T) {
	for _, seed := range []string{"", "other-seed"} {
		if _, err := northstar.Generate(seed); err == nil {
			t.Fatalf("expected error for seed %q", seed)
		}
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
