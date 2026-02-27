package sellers

import (
	"context"
	"fmt"

	supabase "github.com/nedpals/supabase-go"
)

// Address represents a business address.
type Address struct {
	ID         int    `json:"id,omitempty"`
	Street     string `json:"street"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

// AddAddress saves a new business address to the database using the Supabase client.
func AddAddress(ctx context.Context, client *supabase.Client, address Address) (Address, error) {
	var results []Address

	insertData := map[string]string{
		"street":      address.Street,
		"city":        address.City,
		"state":       address.State,
		"postal_code": address.PostalCode,
		"country":     address.Country,
	}

	err := client.DB.From("sellers_business_addresses").Insert(insertData).Execute(&results)
	if err != nil {
		return Address{}, fmt.Errorf("failed to insert address: %w", err)
	}

	if len(results) > 0 {
		return results[0], nil
	}

	return address, nil
}

// GetAddressByID retrieves a single business address from the database by its ID.
func GetAddressByID(ctx context.Context, client *supabase.Client, id string) (Address, error) {
	var results []Address // Query into a slice instead of a single struct

	// Execute the query to find matching addresses.
	err := client.DB.From("sellers_business_addresses").Select("*").Eq("id", id).Execute(&results)
	if err != nil {
		return Address{}, fmt.Errorf("failed to get address by ID: %w", err)
	}

	// Check if any result was found.
	if len(results) == 0 {
		return Address{}, fmt.Errorf("no address found with ID: %s", id)
	}

	// Return the first result from the slice.
	return results[0], nil
}
