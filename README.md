# Web-Crawler

### This is concurrent web crawler written in Go that collects
### webpages data and stores it in Mongodb

## Main Components

1. Queue Implementation (Queue struct)

Thread-safe queue for managing URL's to crawl
Uses a mutex(sync.Mutex) for concurrent access
Methods:
    enqueue(): Adds a URL to the queue
    dequeue(): Removes and returns the first URL
    size(): returns the current queue size

2. CrawledSet Implementation

Tracks already crawled URL's using a map with hash values
Prevents duplicate crawling
Methods:
    add(): Marks a URL as crawled
    contains(): Checks if URL was crawled
    size(): returns count of crawled URLs

3. Database Connection (DatabaseConnection struct)

Manages Mongodb connection
Methods:
    connect(): Establishes connection and clears collection
    disconnect(): closes connection
    insertWebpage(): stores webpages data

4. Webpage Data Structure (Webpage struct)

Store URL, title, and content of crawled pages

5. Crawler Statistics (CrawlerStats struct)

Tracks crawling progress over time
Records pages per minute and crawled-to-queued ratio

## Core Workflow

1. Initialization

Loads environment variables (mongodb uri)
Sets up database connection
Initializes queue with seed URL

2. Crawling Loop

Dequeues a URL
Fetches page content (concurrently via goroutine)
Parses HTML to:
    Extract page title and content (first 500 chars)
    Find new links (only HTTP URL)
    Enqueue new links if not already crawled

Stores pages data in monogodb

3. Statistics Tracking:

Runs in separate goroutine
Update metrics every minute

4. Termination

Stops when either:
    Queue is empty
    5000 pages crawled

Display final statistics


## key Concurrency Features

1. Mutex Protection:
    All shared data structures (Queue, CrawledSet) use mutexes
    for thread safety

2. Goroutines
    Page fetching happens concurrently
    Statistics tracking runs in background

3. Channels:
    Used to receive fetched page content


## HTML Parsing Logic

The parser:
    Skips Javascript and style tags
    Extract page title
    Collects text content (first 500 chars)
    Extracts and validates links (only HTTP/HTTPS)

## Performance Considerations

1.Rate Limiting: No explicit delay between requests
2.Content Limits: Only stores first 500 characters of text content
3.Token Limits: Stops parsing after 5000 tokens to prevent excessive processing

## Output

The Program prints:
    1. Progress during crawling (URL and title)
    2. Final statistics including:
        Total URLs queued
        Remaining queue size
        Total pages crawled
        Pages per minute
        Crawled-to-queued ratio over time

#### This is a robus, concurrent web crawler implementation that balances performance with resource management while providing useful statistics about crawling process