package sellers

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	supabase "github.com/nedpals/supabase-go"
)

// BusinessInformation represents a seller's detailed business information.
type BusinessInformation struct {
	ID           int64     `json:"id,omitempty"`
	SellerID     uuid.UUID `json:"seller_id"`
	BusinessName string    `json:"business_name,omitempty"`
	SellerName   string    `json:"seller_name,omitempty"`
	PANCard      string    `json:"pan_card,omitempty"`
	GSTIN        string    `json:"gstin_no,omitempty"`
	Address      string    `json:"address,omitempty"`
	City         string    `json:"city,omitempty"`
	State        string    `json:"state,omitempty"`
	PinCode      string    `json:"pin_code,omitempty"`
	Country      string    `json:"country,omitempty"`
	WebsiteURL   string    `json:"website_url,omitempty"`
	CreatedAt    string    `json:"created_at,omitempty"`
}

// AddBusinessInfo adds a new business information record for a seller using a map.
func AddBusinessInfo(ctx context.Context, client *supabase.Client, infoData map[string]interface{}) (BusinessInformation, error) {
	var results []BusinessInformation
	err := client.DB.From("business_information").Insert(infoData).Execute(&results)
	if err != nil {
		return BusinessInformation{}, fmt.Errorf("failed to insert business info: %w", err)
	}
	if len(results) > 0 {
		return results[0], nil
	}
	return BusinessInformation{}, fmt.Errorf("no rows returned after insert")
}

// GetBusinessInfo fetches the business information for a given seller ID.
func GetBusinessInfo(ctx context.Context, client *supabase.Client, sellerID uuid.UUID) (BusinessInformation, error) {
	var result []BusinessInformation
	err := client.DB.From("business_information").Select("*").Eq("seller_id", sellerID.String()).Execute(&result)
	if err != nil {
		return BusinessInformation{}, fmt.Errorf("failed to get business info: %w", err)
	}
	if len(result) == 0 {
		return BusinessInformation{}, fmt.Errorf("PGRST116") // Simulate Supabase "no rows found"
	}
	return result[0], nil
}

// UpdateBusinessInfo updates an existing business information record.
func UpdateBusinessInfo(ctx context.Context, client *supabase.Client, sellerID uuid.UUID, updates map[string]interface{}) (BusinessInformation, error) {
	var results []BusinessInformation
	err := client.DB.From("business_information").Update(updates).Eq("seller_id", sellerID.String()).Execute(&results)
	if err != nil {
		return BusinessInformation{}, fmt.Errorf("failed to update business info: %w", err)
	}
	if len(results) == 0 {
		return BusinessInformation{}, fmt.Errorf("no rows returned after update")
	}
	return results[0], nil
}

// DeleteBusinessInfo deletes the business information for a seller.
func DeleteBusinessInfo(ctx context.Context, client *supabase.Client, sellerID uuid.UUID) error {
	var results []interface{}
	err := client.DB.From("business_information").Delete().Eq("seller_id", sellerID.String()).Execute(&results)
	if err != nil {
		return fmt.Errorf("failed to delete business info: %w", err)
	}
	return nil
}
