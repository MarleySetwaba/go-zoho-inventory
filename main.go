package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-zoho-inventory/email"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type TokenStore struct {
	AccessToken string `json:"access_token"`
	ExpiryTime  int64  `json:"expiry_time"`
}

type ZohoResponse struct {
	Code        int           `json:"code"`
	Message     string        `json:"message"`
	Items       []interface{} `json:"items,omitempty"`
	Contacts    []interface{} `json:"contacts,omitempty"`
	PageContext []interface{} `json:"page_context,omitempty"`
	// Add other fields as needed
}

///======================================================================================================================================

///======================================================================================================================================

const (
	zohoTokenURL  = "https://accounts.zoho.com/oauth/v2/token"
	tokenFilePath = "./zoho_tokens.json" // Path to store tokens
)

var (
	zohoClientID     string
	zohoClientSecret string
	zohoOrgID        string
	refresh_token    string
	redirect_uri     string
)

// Run init function to initialize all sensitive information
func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	zohoClientID = os.Getenv("ZOHO_CLIENT_ID")
	zohoClientSecret = os.Getenv("ZOHO_CLIENT_SECRET")
	zohoOrgID = os.Getenv("ZOHO_ORG_ID")
	refresh_token = os.Getenv("REFRESH_TOKEN")
	redirect_uri = os.Getenv("REDIRECT_URI")

	if zohoClientID == "" || zohoClientSecret == "" || zohoOrgID == "" {
		log.Fatal("Missing Zoho credentials in .env file")
	}

}

// refresh token function
func refreshToken() (TokenResponse, error) {

	zohoTokenURL := fmt.Sprintf("https://accounts.zoho.com/oauth/v2/token?refresh_token=%s&client_id=%s&client_secret=%s&redirect_uri=%s&grant_type=%s", refresh_token, zohoClientID, zohoClientSecret, redirect_uri, "refresh_token")
	jsonData, err := json.Marshal(TokenResponse{})
	if err != nil {
		return TokenResponse{}, err
	}

	resp, err := http.Post(zohoTokenURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return TokenResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return TokenResponse{}, fmt.Errorf("Zoho API returned status: %d, body: %s", resp.StatusCode, string(body))
	}

	var tokenResponse TokenResponse
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		return TokenResponse{}, err
	}

	return tokenResponse, nil
}

// load tokens function
func loadTokens() (TokenStore, error) {
	data, err := os.ReadFile(tokenFilePath)
	if err != nil {
		return TokenStore{}, err
	}

	var tokens TokenStore
	err = json.Unmarshal(data, &tokens)
	if err != nil {
		return TokenStore{}, err
	}

	return tokens, nil
}

// save tokens to file function
func saveTokens(tokens TokenStore) error {
	jsonData, err := json.Marshal(tokens)
	if err != nil {
		return err
	}

	err = os.WriteFile(tokenFilePath, jsonData, 0600)
	if err != nil {
		return err
	}

	return nil
}

// gets access tokens from file function
func getAccessToken() (string, error) {
	tokens, err := loadTokens()
	if err != nil {
		return "", fmt.Errorf("tokens not found: %w", err)
	}

	if time.Now().Unix() < tokens.ExpiryTime {
		return tokens.AccessToken, nil
	}

	newToken, err := refreshToken()
	if err != nil {
		return "", err
	}

	tokens.AccessToken = newToken.AccessToken
	tokens.ExpiryTime = time.Now().Unix() + int64(newToken.ExpiresIn)

	err = saveTokens(tokens)
	if err != nil {
		return "", err
	}

	return tokens.AccessToken, nil
}

// zoho request handler
func zohoHandler(c *gin.Context) {
	accessToken, err := getAccessToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get access token: " + err.Error()})
		return
	}

	path := c.Param("path")
	query := c.Request.URL.Query()
	method := c.Request.Method

	zohoURL := fmt.Sprintf("https://www.zohoapis.com/inventory/v1/%s", path)

	reqURL, err := url.Parse(zohoURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid URL"})
		return
	}
	reqURL.RawQuery = query.Encode()

	req, err := http.NewRequest(method, reqURL.String(), c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	req.Header.Set("Authorization", "Zoho-oauthtoken "+accessToken)

	req.Header.Set("Content-Type", "application/json")

	req.Header.Set("X-com-zoho-inventory-organizationid", zohoOrgID)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make API request"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	var zohoResp ZohoResponse
	err = json.Unmarshal(body, &zohoResp)
	if err != nil {
		c.Data(resp.StatusCode, "application/json", body)
		return
	}

	c.JSON(resp.StatusCode, zohoResp)
}

func main() {
	router := gin.Default()
	router.Any("/zoho/*path", zohoHandler)
	router.GET("/vehicles/:id", getModelAndVehicle)
	router.GET("/parts/:id", getVehicleParts)
	router.POST("/email-contact/", emailContact)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server listening on port %s", port)
	router.Run(":" + port)
}

// get Models and Vehicles by manufacturer id
func getModelAndVehicle(c *gin.Context) {
	id := c.Param("id")

	filename := fmt.Sprintf("/vehicles/%s.json", id)
	filePath := filepath.Join("./json", filename)
	c.File(filePath)

}

// get parts using vehicle id
func getVehicleParts(c *gin.Context) {
	id := c.Param("id")

	filename := fmt.Sprintf("/parts/%s.json", id)
	filePath := filepath.Join("./json", filename)
	c.File(filePath)
}

// email contact
func emailContact(c *gin.Context) {
	var newEmailContact email.UserEmail

	if err := c.BindJSON(&newEmailContact); err != nil {
		fmt.Println(err)
		return
	}

	err := email.SendEmail(newEmailContact.User.Email, newEmailContact)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send email: " + err.Error()})
		return
	}

	c.IndentedJSON(http.StatusCreated, gin.H{"success": "Email Sent Successfully"})
}



