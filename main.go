package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yasseen-salama/brezel-crawler/internal/crawler"
)

func main() {
	numCrawlers := 2

	allQueries, err := readQueriesFromFile("data/input/queries.txt")
	if err != nil {
		log.Fatalf("Failed to read queries from file: %v\n", err)
	}

	log.Printf("Read %d queries from file:", len(allQueries))
	for i, query := range allQueries {
		log.Printf("Query %d: %s", i+1, query)
	}

	orchestrator := crawler.NewOrchestrator(numCrawlers)

	// Create a context to handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up a signal handler for graceful shutdown (CTRL+C)
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Println("Shutting down...")
		cancel()
	}()

	// Start the orchestrator in a separate goroutine
	go orchestrator.Start(ctx)

	// Distribute queries among crawlers
	queriesPerCrawler := len(allQueries) / numCrawlers
	jobIDMap := make(map[string]string)

	for i := 0; i < numCrawlers; i++ {
		start := i * queriesPerCrawler
		end := start + queriesPerCrawler
		if i == numCrawlers-1 {
			end = len(allQueries) // Last crawler gets any remaining queries
		}

		crawlerQueries := allQueries[start:end]

		go func(queries []string) {
			for _, query := range queries {
				jobID := orchestrator.SubmitJob(query, 1) // Depth is hardcoded to 1 for now
				jobIDMap[query] = jobID                   // Track the job ID
				log.Printf("Submitted job with ID: %s for query: %s\n", jobID, query)
			}
		}(crawlerQueries)
	}

	// Wait for the jobs to finish (simulate a running application)
	time.Sleep(100 * time.Second)

	// Fetch and print the status of the jobs
	for query, jobID := range jobIDMap {
		status, err := orchestrator.GetJobStatus(jobID)
		if err != nil {
			log.Printf("Failed to get status for job %s: %v\n", jobID, err)
		} else {
			log.Printf("Job %s (query: %s) status: %s\n", jobID, query, status)
		}
	}
}

func readQueriesFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var queries []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			queries = append(queries, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return queries, nil
}
