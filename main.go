package main

import (
	"fmt"
	"sync"
	"hash/fnv"
	"os"
	"io"
	"context"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/html"
)

type Queue struct {
	totalQueued int
	count int
	urls []string
	mx sync.Mutex
}

func (q *Queue) enqueue (url string) {
	q.mx.Lock()
	defer q.mx.Unlock()
	q.urls = append(q.urls, url)
	q.totalQueued++
	q.count++
}

func (q *Queue) dequeue () string {
	q.mx.Lock()
	defer q.mx.Unlock()
	url := q.urls[0]
	q.urls = q.urls[1:]
	q.count--
	return url
}

func (q *Queue) size() int {
	q.mx.Lock()
	defer q.mx.Unlock()
	return q.count
}

func hashIt(url string) uint64 {
	obj := fnv.New64a()
	obj.Write([]byte(url))
	return obj.Sum64()
}

type Crawled struct {
	data map[uint64]bool
	count int
	mx sync.Mutex
}

func (c *Crawled) add(url string) {
	c.mx.Lock()
	defer c.mx.Unlock()
	c.data[hashIt(url)] = true
	c.count++
}

func (c *Crawled) contains(url string) bool {
	c.mx.Lock()
	defer c.mx.Unlock()
	return c.data[hashIt(url)]
}

func (c *Crawled) size() int {
	c.mx.Lock()
	defer c.mx.Unlock()
	return c.count;
}

type DatabaseConnection struct {
	access bool
	uri string
	client *mongo.Client
	collection *mongo.Collection
}

func (d *DatabaseConnection) connect() {
	if d.access {
		// Connect to database
		d.uri = os.Getenv("MONGODB_URI")
		client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(d.uri))
		if err != nil {
			panic(err)
		}
		d.client = client
		d.collection = d.client.Database("webCrawlerArchive").Collection("webpages")
		filter := bson.D{{}}
		// Deletes all documents in the collection
		d.collection.DeleteMany(context.TODO(), filter)
	}
}

func (d *DatabaseConnection) Disconnect() {
	if d.access {
		d.client.Disconnect(context.TODO())
	}
}

func (d *DatabaseConnection) InsertPage(webpage webpage) {
	if d.access {
		d.collection.InsertOne(context.TODO, webpage)
	}
}

func getValidHref(t html.Token) (ok bool, href string) {
	for _, a := range t.Attr {
		// only checking href attributes for the links
		if a.Key == "href" {
			if len(a.Val) == 0 || !strings.HasPrefix(a.Val, "http") {
				ok = false
				href = a.Val
				return ok, href
			}
		}
	}
	ok = true
	href = a.Val
	return ok, href
}

func fetchPage(url string, c chan []byte) {
	res, err := http.GET(url)
	if err != nil {
		body := []byte("")
		c <- body
		fmt.Println("Fail fetching the page")
		return
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		body := []byte("")
		c <- body
		fmt.Println("empty response")
		return
	}
	c <- body
} 

func parseHTML(currentURL string, content []byte, q *Queue, crawled *CrawledSet, db *DatabaseConnection) {
	z := html.NewTokenizer(bytes.NewReader(content))
	tokenCount := 0
	bodyLengthCount := 0
	body := false
	webpage := webpage{Url: currentURL, Title: "", Content: ""}
	for {
		t := z.Token()
		if t.Type == html.StartTagToken {
			if t.Data == "body" {
				body = true
			}

			if t.Data == "javascript" || t.Data == "script" || t.Data == "style" {
				z.Next()
				continue
			}

			if t.Data == "title" {
				z.Next()
				title := z.Token().Data
				webpage.Title = title
				fmt.Printf("Count: %d | %s -> %s\n", crawled.size(), currUrl, title)
			}

			if t.Data == "a" {
				z.Next()
				ok, href := getValidHref(t)

				if !ok {
					continue
				}
				
				if crawled.contains(href) {
					continue
				} else {
					q.enqueue(href)
				}
			}
		}

		if body && t.Type == htmlTextToken && pageContentLength < 500 {
			webpage.Content += strings.TrimSpace(t.Data)
			pageContentLength += len(t.Data)
		} 

		tokenCount++
	}
}

type webpage struct {
	Title string
	Url string 
	Content string
}

func main() {
	webArchiveAccess := true
	if godotenv.Load() != nil {
		fmt.Println("Error loading .env file , failed to access webArchieve access")
		webArchiveAccess = false
	}

	db := DatabaseConnection{access: webArchiveAccess, uri: "", client: nil, collection: nil}

	db.connect()

	crawled := CrawledSet{data: make(map[uint64]bool)}
	seed := "https://www.cc.gatech.edu/"

	queue := Queue{totalQueued: 0, count: 0, uri: make([]string, 0)}

	ticker := time.NewTicker(1 * time.Minute)

	done := make(chan bool)

	crawlStats := CrawledStats{pagesPerMinute: "0 0\n", crawledRatioPerMinute: "0 0\n", startTime: time.Now()}

	// Tick every minute
	go func() {
		for {
            select {
            case <-done:
                return
            case t := <-ticker.C:
                crawlerStats.update(&crawled, &queue, t)
            }
        }
	}()

	queue.enqueue(seed)
	url := queue.dequeue()
	crawled.add(url)
	c := make(chan []byte)

	go fetchPage(url, c)
	content := <-c

	parseHTML(url, content, &queue, &crawled, &db)

	for queue.size() > 0 && crawled.size() < 5000 {
		url := queue.dequeue()
		crawled.add(url)

		go fetchPage(url, c)
		content := <- c
		if len(content) == 0 {
			continue
		}
		go parseHTML(url, content, &queue, &crawled, &db)
	}

	ticker.Stop()
    done <- true
	db.disconnect()
	fmt.Println("\n------------------CRAWLER STATS------------------")
	fmt.Printf("Total queued: %d\n", queue.totalQueued)
	fmt.Printf("To be crawled (Queue) size: %d\n", queue.size())
	fmt.Printf("Crawled size: %d\n", crawled.size())
	crawlerStats.print()
}

type CrawledStats struct {
	pagesPerMinute string
	crawledRatioPerMinute string
	startTime time.Time
}

func (c *CrawledStats) update(crawled *CrawledSet, queue *Queue, t time.Time) {
	c.pagesPerMinute += fmt.Sprintf("%f %d\n", t.Sub(c.startTime).Minutes(), crawled.size())
	c.crawledRatioPerMinute += fmt.Sprintf("%f %f\n", t.Sub(c.startTime).Minutes(), float64(crawled.size())/float64(queue.size()))
}

func (c *CrawledStats) print() {
	fmt.Println("pages crawled per minute")
	fmt.Println(c.pagesPerMinute)
	fmt.Println("Crawled to queue ratio per minute")
	fmt.Println(c.crawledRatioPerMinute)
}