package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var db *sqlx.DB

func initDB() {
	var err error
	db, err = sqlx.Connect("postgres", "host=localhost port=5432 user=admin dbname=zocket sslmode=disable")
	if err != nil {
		panic(err)
	}
	fmt.Println("Database connected")
}

func updateSpend(c *gin.Context) {
	campaignID := c.Param("campaign_id")
	var request struct {
		Spend float64 `json:"spend"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Use a database transaction to ensure atomicity
	tx, err := db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}

	_, err = tx.Exec("UPDATE campaigns SET spend = spend + $1 WHERE id = $2", request.Spend, campaignID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database update failed"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Spend updated"})
}

func getBudgetStatus(c *gin.Context) {
	campaignID := c.Param("campaign_id")
	var budget, spend float64

	err := db.QueryRow("SELECT budget, spend FROM campaigns WHERE id = $1", campaignID).Scan(&budget, &spend)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch campaign data"})
		}
		return
	}

	remaining := budget - spend
	status := "Active"
	if remaining <= 0 {
		status = "Overspent"
	}

	c.JSON(http.StatusOK, gin.H{
		"campaign_id": campaignID,
		"budget":      budget,
		"spend":       spend,
		"remaining":   remaining,
		"status":      status,
	})
}

func main() {
	initDB()
	r := gin.Default()
	r.POST("/campaigns/:campaign_id/spend", updateSpend)
	r.GET("/campaigns/:campaign_id/budget-status", getBudgetStatus)

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080" // Default port if not set
	}

	r.Run(":" + port)
}
