package quota

import (
	"context"
	"testing"
)

func TestMemoryQuotaDailyLimit(t *testing.T) {
	q := NewMemoryQuota()
	ctx := context.Background()

	// daily 100, request estimated 60
	allowed, dr, mr := q.Allow(ctx, "k1", 100, 10000, 60)
	if !allowed || dr != 40 || mr != 9940 {
		t.Fatalf("1st: allowed=%v dailyRem=%d monthlyRem=%d", allowed, dr, mr)
	}
	q.Record(ctx, "k1", 60)

	// daily 100, request estimated 50 -> exceeds remaining 40
	allowed, _, _ = q.Allow(ctx, "k1", 100, 10000, 50)
	if allowed {
		t.Fatal("2nd should be rejected (daily quota exceeded)")
	}
}

func TestMemoryQuotaMonthlyLimit(t *testing.T) {
	q := NewMemoryQuota()
	ctx := context.Background()

	allowed, _, mr := q.Allow(ctx, "k2", 100000, 50, 60)
	if allowed {
		t.Fatalf("should reject: monthly remaining=%d", mr)
	}
}
