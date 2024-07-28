package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"sync"
	"github.com/gin-contrib/cors"
)

type Config struct {
	DataFile      string `json:"data_file"`
	Port          string `json:"port"`
	AdminPassword string `json:"admin_password"`
}

var (
	data   = make(map[string]string)
	dataMu sync.RWMutex
)

func loadData(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// File does not exist, no data to load
			return nil
		}
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(&data)
}

func saveData(filename string) error {
	dataMu.Lock()
	defer dataMu.Unlock()

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	return encoder.Encode(data)
}

func main() {
	configFile := flag.String("config", "config.json", "Path to the configuration file")
	flag.Parse()

	file, err := os.Open(*configFile)
	if err != nil {
		fmt.Printf("Error opening config file: %v\n", err)
		return
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		fmt.Printf("Error decoding config file: %v\n", err)
		return
	}

	if err := loadData(config.DataFile); err != nil {
		fmt.Printf("Error loading data: %v\n", err)
		return
	}

	r := gin.Default()

	r.Use(cors.New(cors.Config{
        AllowOrigins:     []string{"*"}, // Allow all origins
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
        ExposeHeaders:    []string{"Content-Length"},
        AllowCredentials: true,
    }))

	r.POST("/kv/set", func(c *gin.Context) {
		var body struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader != config.AdminPassword {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		dataMu.Lock()
		data[body.Key] = body.Value
		dataMu.Unlock()

		if err := saveData(config.DataFile); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save data"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	r.GET("/kv/get/:key", func(c *gin.Context) {
		key := c.Param("key")

		dataMu.RLock()
		value, exists := data[key]
		dataMu.RUnlock()

		if !exists {
			c.String(http.StatusNotFound, "Key not found")
			return
		}

		c.String(http.StatusOK, value)
	})

	r.GET("/kv/is-valid/:password", func(c *gin.Context) {
		pass := c.Param("password")
		if pass == config.AdminPassword {
			c.JSON(http.StatusOK, gin.H{"status":true})
		} else {
			c.JSON(http.StatusOK, gin.H{"status":true})
		}
	})

	r.GET("/kv/get-keys", func(c *gin.Context) {
		dataMu.RLock()
		keys := make([]string, 0, len(data))
		for key := range data {
			keys = append(keys, key)
		}
		dataMu.RUnlock()

		c.JSON(http.StatusOK, keys)
	})

	fmt.Printf("Starting server on port %s...\n", config.Port)
	r.Run(":" + config.Port)
}
