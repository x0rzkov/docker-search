package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/k0kubun/pp"
)

type WebClient interface {
	Get(string) []byte
}

type Client struct {
	config      *Config
	dockerfiles map[string]string
	Images      []DockerImage
	Results     []DockerImage
	Http        WebClient
	Verbose     bool
}

type Docker struct {
	terms []string
}

func (c *Client) Load(files map[string]string) {
	c.dockerfiles = files
}

type Config struct {
	Host        string
	Endpoint    string
	UpdateCheck bool
}

func (c *Client) log(msg ...string) {
	logit(c.Verbose, msg...)
}

func (c *Client) LoadConfig(path string) bool {
	rv := true
	var conf Config
	if _, err := toml.DecodeFile(path, &conf); err != nil {
		// handle error
		log.Fatal(err)
		rv = false
	} else {
		c.config = &conf
	}
	return rv

}

type Tuple struct {
	Name       string
	Dockerfile string
}

type RealWebClient struct {
}

func (r *RealWebClient) Get(url string) []byte {
	var body []byte
	// Raw link: https://registry.hub.docker.com/u/bfirsh/ffmpeg/dockerfile/raw
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)

	if resp, err := client.Do(req); nil != err {
		log.Fatal(err)
	} else {
		defer resp.Body.Close()
		body, _ = ioutil.ReadAll(resp.Body)
	}
	return body
}

func (c *Client) grabDockerfile(name string) []byte {
	return nil
	url := "https://registry.hub.docker.com/u/" + name + "/dockerfile/raw"
	body := c.Http.Get(url)
	return body
}

func (c *Client) sendBackDockerfile(ci chan<- Tuple, name string) {
	body := c.grabDockerfile(name)
	ci <- Tuple{name, string(body)}
}

func (c *Client) processDockerfile(ci <-chan Tuple) {
	tuple := <-ci
	// Apply it to the correct result
	for i, image := range c.Images {
		if tuple.Name == image.Name {
			c.log("Got dockerfile for: " + tuple.Name)
			c.Images[i].Dockerfile = strings.TrimSpace(html.UnescapeString(tuple.Dockerfile))
		}
	}
}

func (c *Client) Annotate() {

	// Grab a bunch of Dockerfiles, and then process them
	count := 0
	ci := make(chan Tuple, 4)
	for _, image := range c.Images {
		c.log("Annotating image " + image.Name + " with Dockerfile")
		go c.sendBackDockerfile(ci, image.Name)
		count++

	}
	for count > 0 {
		c.processDockerfile(ci)
		count--
	}
	c.log("Finished annotation of dockerfiles")
}

func (c *Client) Filter(filters []string) {
	// Set them to the results
	c.Results = []DockerImage{}
	filterCount := len(filters)
	counts := make(map[string]int)

	if 0 < filterCount {
		c.log("Filtering dockerfiles")

		for _, filter := range filters {

			td := ProcessFilter(filter)
			for _, image := range c.Images {
				if -1 != strings.Index(image.Dockerfile, td.Target) {
					c.log("Found match for filter " + td.Target + " of Dockerfile for image: " + image.Name)
					counts[image.Name] += 1
				}
			}
		}

		for k, v := range counts {
			if v == filterCount { // Every filter matches
				// Find the image and add it
				for _, e := range c.Images {
					if e.Name == k {
						c.Results = append(c.Results, e) // Add it to the results
					}
				}
			}
		}
	} else {
		// No filters, give back all results
		c.Results = c.Images
	}
}

type TargetDescription struct {
	Src     bool
	Version string
	Target  string
}

func ProcessFilter(needle string) *TargetDescription {
	td := new(TargetDescription)
	td.Src = false
	td.Version = ""
	usingColon := strings.Index(needle, ":")
	usingComma := strings.Index(needle, ",")
	if -1 != usingColon || -1 != usingComma {
		// Split it up, using the correct delimiter
		delimiter := ":"
		if -1 != usingComma {
			delimiter = ","
		}
		pieces := strings.Split(needle, delimiter)
		needle = pieces[0]
		for _, e := range pieces[1:] {
			if "src" == e {
				td.Src = true
			} else {
				// assume it is the version
				td.Version = e
			}
		}
	}
	td.Target = needle
	return td
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

func (c *Client) Query(term string) bool {

	rv := true
	// GET /v1/search?q=search_term HTTP/1.1
	// Host: example.com
	// Accept: application/json
	currentPage := 1
	lastPage := 1

	for currentPage <= lastPage {
		url := c.config.Host + c.config.Endpoint + "?q=" + term + "&page=" + fmt.Sprintf("%d", currentPage)
		pp.Println(url)
		body := c.Http.Get(url)
		var res DockerResults
		err := json.Unmarshal(body, &res)
		if nil == err {
			lastPage = res.LastPage
			c.Images = append(c.Images, res.Results...)
			c.Results = append(c.Results, res.Results...)
		} else {
			log.Fatal(err)
		}
		currentPage++
	}
	return rv
}
