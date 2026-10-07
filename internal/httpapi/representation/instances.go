package representation

import ()

// InstanceStatsResponse is the JSON representation of GET /instances/stats:
// the total in scope plus the breakdown by connection status. Unknown
// statuses fold into disconnected so the four buckets always sum to total.
type InstanceStatsResponse struct {
	Total    int            `json:"total" binding:"required"`
	ByStatus map[string]int `json:"by_status" binding:"required"`
}
