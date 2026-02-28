package sellers

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	supabase "github.com/nedpals/supabase-go"
)

// Seller represents a seller's profile linked to an authenticated user.
type Seller struct {
	ID           uuid.UUID `json:"id" db:"id"`
	BusinessName string    `json:"business_name" db:"business_name"`
	BusinessType string    `json:"business_type" db:"business_type"`
	CreatedAt    string    `json:"created_at,omitempty" db:"created_at"`
}

// AddSeller creates a new seller profile using a map of data.
func AddSeller(ctx context.Context, client *supabase.Client, sellerData map[string]interface{}) (Seller, error) {
	var results []Seller

	// Insert the map directly. The supabase-go library handles this well.
	err := client.DB.From("sellers").Insert(sellerData).Execute(&results)
	if err != nil {
		return Seller{}, fmt.Errorf("failed to create seller: %w", err)
	}

	if len(results) == 0 {
		return Seller{}, fmt.Errorf("no data returned after insert")
	}

	return results[0], nil
}

// GetSeller fetches the profile for the given seller ID. RLS should be active.
func GetSeller(ctx context.Context, client *supabase.Client, sellerID uuid.UUID) (Seller, error) {
	var result Seller
	err := client.DB.From("sellers").Select("*").Single().Eq("id", sellerID.String()).Execute(&result)
	if err != nil {
		return Seller{}, fmt.Errorf("failed to fetch seller: %w", err)
	}
	return result, nil
}
