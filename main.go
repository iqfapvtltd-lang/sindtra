package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/GoogleCloudPlatform/golang-samples/run/helloworld/sellers"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	supabase "github.com/nedpals/supabase-go"
)

var supabaseClient *supabase.Client

// addAddressHandler handles the API request to add a new business address.
func addAddressHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Decode the incoming JSON request body.
	var address sellers.Address
	err := json.NewDecoder(r.Body).Decode(&address)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		log.Printf("Error decoding request body: %v", err)
		return
	}

	// 2. Call the logic to save the address to Supabase.
	savedAddress, err := sellers.AddAddress(r.Context(), supabaseClient, address)
	if err != nil {
		http.Error(w, "Failed to save address", http.StatusInternalServerError)
		log.Printf("Error saving address: %v", err)
		return
	}

	// 3. Send the newly created address (with ID) back to the client.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(savedAddress)
}

// getAddressByIDHandler handles the API request to get a business address by its ID.
func getAddressByIDHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Get the address ID from the URL path.
	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		http.Error(w, "Address ID is missing in URL", http.StatusBadRequest)
		return
	}

	// 2. Call the logic to get the address from Supabase.
	address, err := sellers.GetAddressByID(r.Context(), supabaseClient, id)
	if err != nil {
		// A common error here would be if the ID doesn't exist, which might be a 404 Not Found.
		// For simplicity, we'll return a 500 Internal Server Error for any database-related failure.
		http.Error(w, "Failed to get address", http.StatusInternalServerError)
		log.Printf("Error getting address by ID: %v", err)
		return
	}

	// 3. Send the address back to the client.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(address)
}

func main() {
	// Load .env.local file
	// In a production environment like Render, we'll use environment variables directly,
	// so we don't treat an error here as fatal.
	if err := godotenv.Load(".env.local"); err != nil {
		log.Println("Warning: .env.local file not found. Using environment variables.")
	}

	// Initialize Supabase client
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")
	if supabaseURL == "" || supabaseKey == "" {
		log.Fatal("Error: SUPABASE_URL and SUPABASE_KEY must be set.")
	}
	supabaseClient = supabase.CreateClient(supabaseURL, supabaseKey)
	if supabaseClient == nil {
		log.Fatal("Failed to initialize Supabase client")
	}

	fmt.Println("Successfully connected to Supabase!")

	// Initialize router
	r := mux.NewRouter()

	// Define API endpoints
	r.HandleFunc("/sellers/address", addAddressHandler).Methods("POST")
	r.HandleFunc("/sellers/address/{id}", getAddressByIDHandler).Methods("GET")

	// Start server - Modified for Render deployment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port for local development
	}

	fmt.Printf("Server starting on port %s\n", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
