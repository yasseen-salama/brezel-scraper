package crawler

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type CrawlJob struct {
	ID       string
	URL      string
	Depth    int
	Status   string
	Result   string
	Attempts int
}

type Crawler interface {
	Crawl(context.Context, *CrawlJob) error
}

type Orchestrator struct {
	crawlerQueues []chan *CrawlJob
	results       chan *CrawlJob
	crawlers      []Crawler
	activeJobs    map[string]*CrawlJob
	maxAttempts   int
	mu            sync.Mutex
}

// NewOrchestrator creates a new Orchestrator instance with the specified number of crawlers.
func NewOrchestrator(numCrawlers int) *Orchestrator {
	o := &Orchestrator{
		crawlerQueues: make([]chan *CrawlJob, numCrawlers),
		results:       make(chan *CrawlJob, 100),
		crawlers:      make([]Crawler, numCrawlers),
		activeJobs:    make(map[string]*CrawlJob),
		maxAttempts:   3,
	}

	for i := 0; i < numCrawlers; i++ {
		o.crawlerQueues[i] = make(chan *CrawlJob, 100)
		o.crawlers[i] = &GoogleMapsCrawler{queue: o.crawlerQueues[i]}
	}

	return o
}

// Start begins the orchestration process.
func (o *Orchestrator) Start(ctx context.Context) {
	go o.processResults()

	for i, crawler := range o.crawlers {
		go o.runCrawler(ctx, i, crawler)
	}

	<-ctx.Done()
}

// SubmitJob adds a new job to the queue.
func (o *Orchestrator) SubmitJob(url string, depth int) string {
	jobID := uuid.New().String()
	job := &CrawlJob{
		ID:     jobID,
		URL:    url,
		Depth:  depth,
		Status: "queued",
	}

	// Round-robin job distribution
	crawlerIndex := len(o.activeJobs) % len(o.crawlerQueues)
	o.crawlerQueues[crawlerIndex] <- job

	o.mu.Lock()
	o.activeJobs[jobID] = job
	o.mu.Unlock()

	return jobID
}

// GetJobStatus retrieves the status of a job by its ID.
func (o *Orchestrator) GetJobStatus(id string) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if job, exists := o.activeJobs[id]; exists {
		return job.Status, nil
	}
	return "", fmt.Errorf("job not found")
}

// runCrawler runs the crawling job using the provided Crawler.
func (o *Orchestrator) runCrawler(ctx context.Context, id int, crawler Crawler) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-o.crawlerQueues[id]:
			log.Printf("Crawler %d processing job %s", id, job.ID)
			job.Status = "in_progress"
			err := crawler.Crawl(ctx, job)
			if err != nil {
				job.Attempts++
				if job.Attempts < o.maxAttempts {
					log.Printf("Job %s failed, retrying. Attempt %d", job.ID, job.Attempts)
					o.crawlerQueues[id] <- job
				} else {
					job.Status = "failed"
					o.results <- job
				}
			} else {
				job.Status = "completed"
				o.results <- job
			}
		}
	}
}

// processResults processes the results of completed jobs.
func (o *Orchestrator) processResults() {
	for job := range o.results {
		o.mu.Lock()
		delete(o.activeJobs, job.ID)
		o.mu.Unlock()

		log.Printf("Job %s completed. Status: %s", job.ID, job.Status)
		if job.Status == "completed" {
			resultsFile := fmt.Sprintf("data/output/result-%s.csv", job.ID)
			err := os.WriteFile(resultsFile, []byte(job.Result), 0644)
			if err != nil {
				log.Printf("Failed to write results for job %s: %v", job.ID, err)
			} else {
				log.Printf("Results for job %s written to %s", job.ID, resultsFile)
			}
		}
	}
}

// GoogleMapsCrawler represents the actual Google Maps scraper crawler.
type GoogleMapsCrawler struct {
	queue chan *CrawlJob
}

// Crawl runs the Google Maps Scraper using the Docker command for the provided job.
func (g *GoogleMapsCrawler) Crawl(ctx context.Context, job *CrawlJob) error {
	log.Printf("Starting crawl for job %s with query: %s", job.ID, job.URL)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %v", err)
	}

	tempQueryFile := filepath.Join(cwd, "data", "input", fmt.Sprintf("temp-query-%s.txt", job.ID))
	resultsFile := filepath.Join(cwd, "data", "output", fmt.Sprintf("result-%s.csv", job.ID))

	log.Printf("Writing query to temp file: %s", tempQueryFile)
	err = os.WriteFile(tempQueryFile, []byte(job.URL), 0644)
	if err != nil {
		return fmt.Errorf("failed to write temporary query file: %v", err)
	}
	defer os.Remove(tempQueryFile)

	// Ensure the output directory exists
	err = os.MkdirAll(filepath.Dir(resultsFile), 0755)
	if err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Create an empty results file
	emptyFile, err := os.Create(resultsFile)
	if err != nil {
		return fmt.Errorf("failed to create results file: %v", err)
	}
	emptyFile.Close()

	dockerArgs := []string{
		"run",
		"--platform", "linux/amd64",
		"-v", fmt.Sprintf("%s:/query.txt:ro", tempQueryFile),
		"-v", fmt.Sprintf("%s:/results.csv", resultsFile),
		"gosom/google-maps-scraper",
		"-depth", fmt.Sprintf("%d", job.Depth),
		"-input", "/query.txt",
		"-results", "/results.csv",
		"-exit-on-inactivity", "3m",
	}

	log.Printf("Executing Docker command for job %s: docker %s", job.ID, strings.Join(dockerArgs, " "))

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Docker command failed for job %s. Error: %v", job.ID, err)
		log.Printf("Docker command output:\n%s", string(output))
		return fmt.Errorf("Google Maps Scraper failed: %v, output: %s", err, string(output))
	}

	log.Printf("Docker command completed for job %s. Output:\n%s", job.ID, string(output))

	log.Printf("Reading results file for job %s: %s", job.ID, resultsFile)
	resultData, err := os.ReadFile(resultsFile)
	if err != nil {
		log.Printf("Failed to read results file for job %s: %v", job.ID, err)
		return fmt.Errorf("failed to read results file: %v", err)
	}

	job.Result = string(resultData)
	log.Printf("Job %s completed successfully. Result length: %d bytes", job.ID, len(job.Result))

	return nil
}
