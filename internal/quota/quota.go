package quota

import (
	"context"
	"sync"
)

// QuotaStore tracks daily/monthly token usage per API key
type QuotaStore interface {
	Allow(ctx context.Context, keyID string, dailyLimit, monthlyLimit, estimated int64) (bool, int64, int64)
	Record(ctx context.Context, keyID string, tokens int64)
}

type usage struct {
	daily   int64
	monthly int64
}

// MemoryQuota is an in-memory quota tracker
type MemoryQuota struct {
	mu    sync.Mutex
	usage map[string]*usage
}

func NewMemoryQuota() *MemoryQuota {
	return &MemoryQuota{usage: make(map[string]*usage)}
}

// Allow checks whether estimated tokens fit within daily/monthly limits.
// On success it returns the remaining quota after reserving this request's
// estimate; on rejection it returns the current remaining quota.
func (q *MemoryQuota) Allow(ctx context.Context, keyID string, dailyLimit, monthlyLimit, estimated int64) (bool, int64, int64) {
	q.mu.Lock()
	defer q.mu.Unlock()

	u, exists := q.usage[keyID]
	if !exists {
		u = &usage{}
		q.usage[keyID] = u
	}

	dailyRem := dailyLimit - u.daily
	monthlyRem := monthlyLimit - u.monthly

	if dailyLimit > 0 && dailyRem < estimated {
		return false, dailyRem, monthlyRem
	}
	if monthlyLimit > 0 && monthlyRem < estimated {
		return false, dailyRem, monthlyRem
	}
	return true, dailyRem - estimated, monthlyRem - estimated
}

// Record accumulates tokens consumed for a key
func (q *MemoryQuota) Record(ctx context.Context, keyID string, tokens int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	u, exists := q.usage[keyID]
	if !exists {
		u = &usage{}
		q.usage[keyID] = u
	}
	u.daily += tokens
	u.monthly += tokens
}
