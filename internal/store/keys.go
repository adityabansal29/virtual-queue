package store

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Redis key constructors — single source of truth for all key namespaces.
// All packages must call these instead of building key strings inline.

func QueueKey(eventID string) string            { return "queue:" + eventID }
func AdmissionRateKey(eventID string) string    { return "rate:" + eventID }
func ActiveSessionCountKey(eventID string) string   { return "active:" + eventID }
func MaxAllowedSessionCountKey(eventID string) string { return "capacity:" + eventID }
func TicketKey(ticketID string) string          { return "ticket:" + ticketID }
func TicketUpdatesKey(ticketID string) string   { return "ticket:updates:" + ticketID }
func QueueTickKey(eventID string) string        { return "queue:tick:" + eventID }
func SchedulerLockKey(eventID string) string    { return "scheduler:lock:" + eventID }

// GetAllEventIDs returns all active event IDs by scanning queue:* keys.
// Returns an empty (non-nil) slice when no events exist.
func GetAllEventIDs(ctx context.Context, rdb *redis.Client) ([]string, error) {
	var cursor uint64
	prefix := QueueKey("")
	var ids []string
	for {
		keys, next, err := rdb.Scan(ctx, cursor, QueueKey("*"), 100).Result()
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			ids = append(ids, k[len(prefix):])
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}
