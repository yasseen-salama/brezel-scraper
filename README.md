# Brezel Crawler

Go based manager for Google Maps scraping tasks. It distributes and processes multiple crawling jobs concurrently.

## Prerequisites

- Go 1.16 or later
- Docker
- Git

## Quick Start

1. Clone the repository:

```bash
git clone https://github.com/your-username/brezel-crawler.git
cd brezel-crawler
```

2. Clone and build the Google Maps Scraper Docker image:

```bash
git clone https://github.com/gosom/google-maps-scraper.git
cd google-maps-scraper
docker build -t gosom/google-maps-scraper .
cd ..
```

3. Build the brezel project:

```bash
go build -o brezel-crawler
```

4. Prepare your input:

Create a file `data/input/queries.txt` with your search queries, one per line. For example:

```
restaurant in nicosia cyprus
restaurant in limassol cyprus
restaurant in paphos cyprus
```

5. Run the crawler:

```bash
./brezel-crawler
```

## Project Structure

- `main.go`: Entry point of the app
- `internal/crawler/orchestrator.go`: Contains the core logic for job distribution and running the crawler via docker 
- `data/input/`:  input files
- `data/output/`: result files  

## Note

This project uses the Google Maps Scraper from https://github.com/gosom/google-maps-scraper to perform the actual scraping tasks. You need to build the Docker image for this scraper locally, as shown above!
