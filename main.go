package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	supabase "github.com/nedpals/supabase-go"

	"sindtra/sellers" // Import our local packages
)

var supabaseClient *supabase.Client

func main() {
	err := godotenv.Load(".env.local")
	if err != nil {
		log.Fatal("Error loading .env.local file")
	}

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")
	supabaseClient = supabase.CreateClient(supabaseURL, supabaseKey)

	r := gin.Default()

	// DEV: Using a temporary middleware for development that doesn't require a JWT
	r.Use(devAuthMiddleware())
	// r.Use(authMiddleware()) // PRODUCTION: This line should be used in production

	// =========== Seller Profile Routes ===========
	sellerRoutes := r.Group("/sellers")
	{
		// POST /sellers - Create a new seller profile
		sellerRoutes.POST("/", handleAddSeller)
		// GET /sellers - Get the current seller's profile
		sellerRoutes.GET("/", handleGetSeller)

		// =========== Business Information Routes for the seller ===========
		bizInfoRoutes := sellerRoutes.Group("/business-information")
		{
			// POST /sellers/business-information - Add business info for the current seller
			bizInfoRoutes.POST("/", handleAddBusinessInfo)
			// GET /sellers/business-information - Get business info for the current seller
			bizInfoRoutes.GET("/", handleGetBusinessInfo)
			// PUT /sellers/business-information - Update business info for the current seller
			bizInfoRoutes.PUT("/", handleUpdateBusinessInfo)
			// DELETE /sellers/business-information - Delete business info for the current seller
			bizInfoRoutes.DELETE("/", handleDeleteBusinessInfo)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}

// ================== TEMP DEV AUTH MIDDLEWARE ==================

func devAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// This is a temporary user ID for development purposes.
		// Replace this with the actual user ID you want to test with.
		testUserID := "69824941-fd32-4548-8c7e-83085a811d24"
		c.Set("userID", testUserID)
		c.Next()
	}
}

// ================== AUTH MIDDLEWARE (FOR PRODUCTION) ==================

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token required"})
			c.Abort()
			return
		}

		// Assumes Bearer token
		const bearerPrefix = "Bearer "
		if len(token) > len(bearerPrefix) {
			token = token[len(bearerPrefix):]
		}

		ctx := context.Background()
		user, err := supabaseClient.Auth.User(ctx, token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token", "details": err.Error()})
			c.Abort()
			return
		}

		// Set the user ID in the context for handlers to use
		c.Set("userID", user.ID)
		c.Next()
	}
}

// ================== HANDLER FUNCTIONS ==================

// ---- Seller Handlers ----

func handleAddSeller(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User ID is not a string"})
		return
	}

	var sellerData map[string]interface{}
	if err := c.ShouldBindJSON(&sellerData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Ensure the seller ID matches the authenticated user's ID
	sellerData["id"] = userIDStr

	createdSeller, err := sellers.AddSeller(c.Request.Context(), supabaseClient, sellerData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create seller profile", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, createdSeller)
}

func handleGetSeller(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)
	parsedUserID, _ := uuid.Parse(userIDStr)

	seller, err := sellers.GetSeller(c.Request.Context(), supabaseClient, parsedUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get seller profile", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, seller)
}

// ---- Business Info Handlers ----

func handleAddBusinessInfo(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User ID is not a string"})
		return
	}

	var infoData map[string]interface{}
	if err := c.ShouldBindJSON(&infoData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Set the seller_id from the authenticated user
	infoData["seller_id"] = userIDStr

	createdInfo, err := sellers.AddBusinessInfo(c.Request.Context(), supabaseClient, infoData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add business information", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, createdInfo)
}

func handleGetBusinessInfo(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)
	parsedUserID, _ := uuid.Parse(userIDStr)

	info, err := sellers.GetBusinessInfo(c.Request.Context(), supabaseClient, parsedUserID)
	if err != nil {
		// Check for a specific error indicating no rows found, which is a valid case
		if err.Error() == "PGRST116" { // Supabase code for no rows found
			c.JSON(http.StatusNotFound, gin.H{"error": "No business information found for this seller"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get business information", "details": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, info)
}

func handleUpdateBusinessInfo(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)
	parsedUserID, _ := uuid.Parse(userIDStr)

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Security: remove id and seller_id from updates to prevent hijacking
	delete(updates, "id")
	delete(updates, "seller_id")

	updatedInfo, err := sellers.UpdateBusinessInfo(c.Request.Context(), supabaseClient, parsedUserID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update business information", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updatedInfo)
}

func handleDeleteBusinessInfo(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)
	parsedUserID, _ := uuid.Parse(userIDStr)

	err := sellers.DeleteBusinessInfo(c.Request.Context(), supabaseClient, parsedUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete business information", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Business information deleted successfully"})
}
