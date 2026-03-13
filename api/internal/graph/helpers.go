package graph

import (
	"github.com/commerce-projects/gitstore/api/internal/graph/model"
	"github.com/google/uuid"
)

// Helper functions for GraphQL resolvers

func generateID() string {
	return uuid.New().String()
}

func stringOrDefault(s *string, def string) string {
	if s != nil {
		return *s
	}
	return def
}

func floatOrDefault(f *float64, def float64) float64 {
	if f != nil {
		return *f
	}
	return def
}

func intOrDefault(i *int32, def int32) int32 {
	if i != nil {
		return *i
	}
	return def
}

func derefInventoryStatus(s *model.InventoryStatus) model.InventoryStatus {
	if s != nil {
		return *s
	}
	return model.InventoryStatusInStock
}
