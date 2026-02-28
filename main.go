
package main

import (
	"context"
    
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	supabase "github.com/nedpals/supabase-go"

	"sindtra/sellers" // Import our local packages
)

var (
	supabaseClient *supabase.Client
	redisClient    *redis.Client
)

func main() {
	// Load environment variables
	err := godotenv.Load(".env.local")
	if err != nil {
		log.Println("Warning: .env.local file not found, relying on Render environment variables")
	}

	// Initialize Supabase client
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")
	if supabaseURL == "" || supabaseKey == "" {
		log.Fatal("SUPABASE_URL and SUPABASE_KEY must be set")
	}
	supabaseClient = supabase.CreateClient(supabaseURL, supabaseKey)

	// Initialize Upstash Redis client
	redisURL := os.Getenv("UPSTASH_REDIS_URL")
	if redisURL == "" {
		log.Fatal("UPSTASH_REDIS_URL must be set")
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Could not parse Redis URL: %v", err)
	}

	redisClient = redis.NewClient(opts)

	// Ping Redis to check the connection
	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	log.Println("Successfully connected to Upstash Redis")


	// Setup Gin router
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

	infoData["seller_id"] = userIDStr

	createdInfo, err := sellers.AddBusinessInfo(c.Request.Context(), supabaseClient, infoData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add business information", "details": err.Error()})
		return
	}
    
    // Invalidate cache after adding new info
    cacheKey := "business_info:" + userIDStr
    redisClient.Del(c.Request.Context(), cacheKey)

	c.JSON(http.StatusCreated, createdInfo)
}

func handleGetBusinessInfo(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)
	ctx := c.Request.Context()
	cacheKey := "business_info:" + userIDStr

	// 1. Check cache first
	cachedData, err := redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache Hit
		log.Println("Cache hit for user:", userIDStr)
		var info sellers.BusinessInformation
		if err := json.Unmarshal([]byte(cachedData), &info); err == nil {
			c.JSON(http.StatusOK, info)
			return
		}
	}
    if err != redis.Nil {
        log.Printf("Redis error (non-Nil) for user %s: %v", userIDStr, err)
    }

	// 2. Cache Miss: Get from database
	log.Println("Cache miss for user:", userIDStr)
	parsedUserID, _ := uuid.Parse(userIDStr)
	info, err := sellers.GetBusinessInfo(ctx, supabaseClient, parsedUserID)
	if err != nil {
		if err.Error() == "PGRST116" {
			c.JSON(http.StatusNotFound, gin.H{"error": "No business information found for this seller"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get business information", "details": err.Error()})
		}
		return
	}

	// 3. Store in cache for next time
	jsonData, err := json.Marshal(info)
	if err != nil {
		log.Printf("Error marshaling business info for caching: %v", err)
	} else {
		// Set cache with a 10-minute expiration
		err := redisClient.Set(ctx, cacheKey, jsonData, 10*time.Minute).Err()
		if err != nil {
			log.Printf("Failed to set cache for user %s: %v", userIDStr, err)
		}
	}

	c.JSON(http.StatusOK, info)
}

func handleUpdateBusinessInfo(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)
    ctx := c.Request.Context()
	parsedUserID, _ := uuid.Parse(userIDStr)

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	delete(updates, "id")
	delete(updates, "seller_id")

	updatedInfo, err := sellers.UpdateBusinessInfo(ctx, supabaseClient, parsedUserID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update business information", "details": err.Error()})
		return
	}

	// Invalidate cache on update
	cacheKey := "business_info:" + userIDStr
	if err := redisClient.Del(ctx, cacheKey).Err(); err != nil {
        log.Printf("Failed to invalidate cache for user %s after update: %v", userIDStr, err)
    }

	c.JSON(http.StatusOK, updatedInfo)
}

func handleDeleteBusinessInfo(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)
    ctx := c.Request.Context()
	parsedUserID, _ := uuid.Parse(userIDStr)

	err := sellers.DeleteBusinessInfo(ctx, supabaseClient, parsedUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete business information", "details": err.Error()})
		return
	}

	// Invalidate cache on delete
	cacheKey := "business_info:" + userIDStr
    if err := redisClient.Del(ctx, cacheKey).Err(); err != nil {
        log.Printf("Failed to invalidate cache for user %s after delete: %v", userIDStr, err)
    }

	c.JSON(http.StatusOK, gin.H{"message": "Business information deleted successfully"})
}
