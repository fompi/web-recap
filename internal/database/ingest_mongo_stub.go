//go:build nomongo

package database

import (
	"context"
	"fmt"

	"github.com/rzolkos/web-recap/internal/models"
)

func ingestMongoDB(_ context.Context, _ string, _ []models.HistoryEntry, _, _ string, _ bool) (int, error) {
	return 0, fmt.Errorf("MongoDB support was not compiled in (rebuild without the 'nomongo' tag)")
}
