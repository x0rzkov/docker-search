package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	badger "github.com/dgraph-io/badger"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/proxy"
	"github.com/gocolly/colly/v2/queue"
)

type Search struct {
	Keywords []string `json:"keywords"`
}

type DockerResults struct {
	Results    []DockerImage `json:"results"`
	Query      string        `json:"query"`
	LastPage   int           `json:"num_pages"`
	NumResults int           `json:"num_results"`
	PageSize   int           `json:"page_size"`
}

type DockerImage struct {
	Description string
	IsOfficial  bool `json:"is_official"`
	IsTrusted   bool `json:"is_trusted"`
	Name        string
	StarCount   int    `json:"star_count"`
	Dockerfile  string `json:"dockerfile"`
}

type DockerFile struct {
	Contents string `json:"contents"`
}

func main() {

	storagePath := flag.String("storage", "./data/docker-search-colly", "storage path")
	torProxy := flag.Bool("tor", false, "tor-proxy")
	flag.Parse()

	store, err := badger.Open(badger.DefaultOptions(*storagePath))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	// Open our jsonFile
	filePath := "search-terms.json"
	jsonFile, err := os.Open(filePath)
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Successfully Opened file: ", filePath)
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var search Search
	err = json.Unmarshal(byteValue, &search)
	if err != nil {
		log.Fatalf("cannot unmarshal data: %v\n", err)
	}

	// Instantiate default collector
	c := colly.NewCollector(
		// Cache responses to prevent multiple download of pages
		// even if the collector is restarted
		colly.CacheDir("./data/cache"),
	)

	if *torProxy {
		rp, err := proxy.RoundRobinProxySwitcher("socks5://127.0.0.1:9050")
		if err != nil {
			log.Fatal(err)
		}
		c.SetProxyFunc(rp)
	}

	// create a request queue with 2 consumer threads
	q, _ := queue.New(
		10, // Number of consumer threads
		&queue.InMemoryQueueStorage{MaxSize: 1500000}, // Use default queue storage
	)

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("visiting", r.URL)
	})

	c.OnResponse(func(r *colly.Response) {
		if *torProxy {
			log.Printf("Proxy Address: %s\n", r.Request.ProxyURL)
		}
		fmt.Println(r.Ctx.Get("url"))
		currentPage := 1

		if strings.HasPrefix(r.Request.URL.String(), "https://hub.docker.com/v2/repositories") {
			var dockerfile DockerFile
			err := json.Unmarshal(r.Body, &dockerfile)
			if err != nil {
				log.Fatalln("error json", err)
			}
			image := strings.Replace(r.Request.URL.String(), "https://hub.docker.com/v2/repositories/", "", -1)
			image = strings.Replace(image, "dockerfile/", "", -1)
			if dockerfile.Contents != "" {
				err = store.Update(func(txn *badger.Txn) error {
					log.Println("indexing dockerfile ", image)
					// log.Println("dockerfile: \n", dockerfile.Contents)
					err := txn.Set([]byte(image+"/dockerfile"), []byte(dockerfile.Contents))
					return err
				})
				if err != nil {
					log.Fatalln("error badger", err)
				}
			}
			return
		}

		var res DockerResults
		err := json.Unmarshal(r.Body, &res)

		lastPage := res.LastPage
		for _, result := range res.Results {
			dockerfile := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/dockerfile/", result.Name)
			// log.Println("enqueuing dockerfile", dockerfile)
			q.AddURL(dockerfile)
			err := store.Update(func(txn *badger.Txn) error {
				// log.Println("indexing ", result.Name)
				err := txn.Set([]byte(result.Name), []byte(result.Description))
				return err
			})
			if err != nil {
				log.Fatalln("error badger", err)
			}
		}

		for currentPage <= lastPage {
			// var res DockerResults
			// err := json.Unmarshal(r.Body, &res)
			if nil == err {
				lastPage = res.LastPage
				// fmt.Println(fmt.Sprintf("%s&page=%v", r.Request.URL, currentPage))
				q.AddURL(fmt.Sprintf("%s&page=%v", r.Request.URL, currentPage))
				// c.Images = append(c.Images, res.Results...)
				// c.Results = append(c.Results, res.Results...)
			} else {
				fmt.Println("error: ", string(r.Body))
				log.Fatalln("error json", err)
			}
			currentPage++
		}

		// log.Printf("%s\n", bytes.Replace(r.Body, []byte("\n"), nil, -1))
	})

	for _, keyword := range search.Keywords {
		// Add URLs to the queue
		q.AddURL(fmt.Sprintf("https://index.docker.io/v1/search?q=%s&n=1000", keyword))
	}
	// Consume URLs
	q.Run(c)

}
