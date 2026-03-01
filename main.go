
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	supabase "github.com/nedpals/supabase-go"
	"google.golang.org/api/option"

	"sindtra/sellers" // Re-importing our local packages
)

var (
	supabaseClient     *supabase.Client
	redisClient        *redis.Client
	firebaseAuthClient *auth.Client
)

func main() {
	// Load environment variables
	err := godotenv.Load(".env.local")
	if err != nil {
		log.Println("Warning: .env.local file not found, relying on Render environment variables")
	}

	// ========== Initialize Firebase Admin SDK ==========
	ctx := context.Background()
	var opt option.ClientOption

    // Priority 1: Use environment variable for production (Render, etc.)
	credentialsJSON := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS_JSON")
	if credentialsJSON != "" {
		opt = option.WithCredentialsJSON([]byte(credentialsJSON))
		log.Println("Initializing Firebase with credentials from environment variable.")
	} else {
        // Priority 2: Fallback to local file for IDX development
        opt = option.WithCredentialsFile("firebase-service-account.json")
		log.Println("Initializing Firebase with credentials from local file.")
    }

	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		log.Fatalf("error initializing Firebase app: %v\n", err)
	}

	firebaseAuthClient, err = app.Auth(ctx)
	if err != nil {
		log.Fatalf("error getting Firebase Auth client: %v\n", err)
	}
	log.Println("Successfully connected to Firebase Auth")

	// ========== Initialize Supabase client ==========
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")
	if supabaseURL == "" || supabaseKey == "" {
		log.Fatal("SUPABASE_URL and SUPABASE_KEY must be set")
	}
	supabaseClient = supabase.CreateClient(supabaseURL, supabaseKey)
    log.Println("Successfully connected to Supabase")

	// ========== Initialize Upstash Redis client ==========
	redisURL := os.Getenv("UPSTASH_REDIS_URL")
	if redisURL == "" {
		log.Println("UPSTASH_REDIS_URL not set, Redis client not initialized.")
	} else {
        opts, err := redis.ParseURL(redisURL)
        if err != nil {
            log.Fatalf("Could not parse Redis URL: %v", err)
        }
        redisClient = redis.NewClient(opts)
        if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
            log.Fatalf("Could not connect to Redis: %v", err)
        }
        log.Println("Successfully connected to Upstash Redis")
    }

	// ========== Setup Gin router ==========
	r := gin.Default()

	// Serve static files from the "public" directory
	r.Static("/public", "./public")

	// Redirect root to the main login/index page
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/public/index.html")
	})

	// All routes under "/api" will be protected by Firebase Auth
	apiRoutes := r.Group("/api")
	apiRoutes.Use(firebaseAuthMiddleware())
	{
		// =========== Seller Profile Routes ===========
		sellerRoutes := apiRoutes.Group("/sellers")
		{
			// POST /api/sellers - Create a new seller profile
			sellerRoutes.POST("/", handleAddSeller)
			// GET /api/sellers - Get the current seller's profile
			sellerRoutes.GET("/", handleGetSeller)

			// =========== Business Information Routes for the seller ===========
			bizInfoRoutes := sellerRoutes.Group("/business-information")
			{
				// POST /api/sellers/business-information - Add business info for the current seller
				bizInfoRoutes.POST("/", handleAddBusinessInfo)
				// GET /api/sellers/business-information - Get business info for the current seller
				bizInfoRoutes.GET("/", handleGetBusinessInfo)
				// PUT /api/sellers/business-information - Update business info for the current seller
				bizInfoRoutes.PUT("/", handleUpdateBusinessInfo)
				// DELETE /api/sellers/business-information - Delete business info for the current seller
				bizInfoRoutes.DELETE("/", handleDeleteBusinessInfo)
			}
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("\033[1;36m%s\033[0m", "\n🚀 Server starting on http://localhost:"+port)
	r.Run(":" + port)
}

// ================== FIREBASE AUTH MIDDLEWARE ==================
func firebaseAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		// The token is expected to be in the format "Bearer <token>"
		idToken := strings.TrimSpace(strings.Replace(authHeader, "Bearer", "", 1))
		if idToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token is missing"})
			c.Abort()
			return
		}

		// Verify the ID token
		token, err := firebaseAuthClient.VerifyIDToken(c, idToken)
		if err != nil {
			log.Printf("Error verifying Firebase ID token: %v\n", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired authorization token"})
			c.Abort()
			return
		}

		// Set the user's Firebase UID in the context for downstream handlers
		c.Set("userID", token.UID)
		c.Next()
	}
}

// ================== HANDLER FUNCTIONS (Original Logic Preserved) ==================

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

	seller, err := sellers.GetSeller(c.Request.Context(), supabaseClient, userIDStr)
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
    if redisClient != nil {
        cacheKey := "business_info:" + userIDStr
        redisClient.Del(c.Request.Context(), cacheKey)
    }

	c.JSON(http.StatusCreated, createdInfo)
}

func handleGetBusinessInfo(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)
	ctx := c.Request.Context()
	
    if redisClient != nil {
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
    }

	// 2. Cache Miss: Get from database
	log.Println("Cache miss for user:", userIDStr)
	info, err := sellers.GetBusinessInfo(ctx, supabaseClient, userIDStr)
	if err != nil {
		if strings.Contains(err.Error(), "PGRST116") { // More robust error check
			c.JSON(http.StatusNotFound, gin.H{"error": "No business information found for this seller"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get business information", "details": err.Error()})
		}
		return
	}

	// 3. Store in cache for next time
    if redisClient != nil {
        jsonData, err := json.Marshal(info)
        if err != nil {
            log.Printf("Error marshaling business info for caching: %v", err)
        } else {
            cacheKey := "business_info:" + userIDStr
            // Set cache with a 10-minute expiration
            err := redisClient.Set(ctx, cacheKey, jsonData, 10*time.Minute).Err()
            if err != nil {
                log.Printf("Failed to set cache for user %s: %v", userIDStr, err)
            }
        }
    }

	c.JSON(http.StatusOK, info)
}

func handleUpdateBusinessInfo(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)
    ctx := c.Request.Context()

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	delete(updates, "id")
	delete(updates, "seller_id")

	updatedInfo, err := sellers.UpdateBusinessInfo(ctx, supabaseClient, userIDStr, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update business information", "details": err.Error()})
		return
	}

	// Invalidate cache on update
    if redisClient != nil {
        cacheKey := "business_info:" + userIDStr
        if err := redisClient.Del(ctx, cacheKey).Err(); err != nil {
            log.Printf("Failed to invalidate cache for user %s after update: %v", userIDStr, err)
        }
    }

	c.JSON(http.StatusOK, updatedInfo)
}

func handleDeleteBusinessInfo(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)
    ctx := c.Request.Context()

	err := sellers.DeleteBusinessInfo(ctx, supabaseClient, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete business information", "details": err.Error()})
		return
	}

	// Invalidate cache on delete
    if redisClient != nil {
        cacheKey := "business_info:" + userIDStr
        if err := redisClient.Del(ctx, cacheKey).Err(); err != nil {
            log.Printf("Failed to invalidate cache for user %s after delete: %v", userIDStr, err)
        }
    }

	c.JSON(http.StatusOK, gin.H{"message": "Business information deleted successfully"})
}
