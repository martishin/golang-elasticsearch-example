package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

type LogEntry struct {
	Service   string    `json:"service"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	// Get Elasticsearch connection string from environment variable
	esURL := os.Getenv("ELASTICSEARCH_URL")

	// Connect to the Elasticsearch cluster
	es, err := elasticsearch.NewClient(
		elasticsearch.Config{
			Addresses: []string{esURL},
		},
	)
	if err != nil {
		log.Fatalf("Error creating the Elasticsearch client: %v", err)
	}

	// Simulate logging data from a microservice
	logEntries := []LogEntry{
		{"auth-service", "INFO", "User logged in successfully", time.Now()},
		{"auth-service", "ERROR", "Failed to authenticate user", time.Now()},
		{"order-service", "INFO", "Order placed successfully", time.Now()},
		{"order-service", "ERROR", "Failed to process order payment", time.Now()},
		{"inventory-service", "INFO", "Inventory updated", time.Now()},
		{"inventory-service", "WARN", "Inventory low for product 1234", time.Now()},
	}

	indexName := "logs"

	// Index the log entries into Elasticsearch
	for _, entry := range logEntries {
		data, err := json.Marshal(entry)
		if err != nil {
			log.Fatalf("Error marshaling log entry: %v", err)
		}

		req := esapi.IndexRequest{
			Index:   indexName,
			Body:    bytes.NewReader(data),
			Refresh: "true",
		}

		res, err := req.Do(context.Background(), es)
		if err != nil {
			log.Fatalf("Error indexing log entry: %v", err)
		}
		defer res.Body.Close()

		if res.IsError() {
			log.Printf("[%s] Error indexing document ID=%d", res.Status(), entry)
		} else {
			log.Printf("Successfully indexed log entry: %s", entry.Message)
		}
	}

	// Query Elasticsearch for errors in the logs
	var buf bytes.Buffer
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match": map[string]interface{}{
				"level": "ERROR",
			},
		},
	}
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		log.Fatalf("Error encoding query: %v", err)
	}

	// Perform the search query
	res, err := es.Search(
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(indexName),
		es.Search.WithBody(&buf),
		es.Search.WithTrackTotalHits(true),
		es.Search.WithPretty(),
	)
	if err != nil {
		log.Fatalf("Error getting response: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Fatalf("Error response from Elasticsearch: %v", res.String())
	}

	// Print the search results
	fmt.Println("Errors in the logs:")
	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		log.Fatalf("Error parsing the response body: %v", err)
	}

	for _, hit := range result["hits"].(map[string]interface{})["hits"].([]interface{}) {
		source := hit.(map[string]interface{})["_source"]
		fmt.Printf(
			"Service: %s, Message: %s, Timestamp: %s\n",
			source.(map[string]interface{})["service"],
			source.(map[string]interface{})["message"],
			source.(map[string]interface{})["timestamp"],
		)
	}
}
