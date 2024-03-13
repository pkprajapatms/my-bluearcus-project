package clearredis

import (
	"bluearcus-backend/inits"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// clearCache handles the request to clear the cache.
func ClearCache(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Clear the cache
	redisClient := inits.GetRedisClient()
	err := redisClient.FlushAll(ctx).Err()
	if err != nil {
		http.Error(w, "Error clearing cache", http.StatusInternalServerError)
		return
	}

	// Respond with a success message
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Cache cleared successfully"})
}
