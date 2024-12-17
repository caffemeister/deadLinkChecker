package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly"
)

// This thing doesn't work if something like Cloudflare is used on the website to stop bots & whatnot. Also doesn't work with JS websites.

func askForUrl() string {
	var url string
	fmt.Println("Let's check for dead links! Enter your URL here: ")
	_, err := fmt.Scanln(&url)
	checkNilErr(err)
	return url
}

// Asks user if external domains should be checked
func askCheckExtDomain() bool {
	var reply string
	fmt.Println("Check external domains? y/n [n will only check the entered host domain]")
	_, err := fmt.Scanln(&reply)
	checkNilErr(err)
	switch strings.ToLower(reply) {
	case "y":
		return true
	case "n":
		return false
	default:
		fmt.Println("Invalid input. Defaulting to 'n'.")
		return false
	}
}

// Checks link status and returns status code
func checkLink(link string) (int, error) {
	r, err := http.Get(link)
	if err != nil {
		return 0, err
	}
	defer r.Body.Close()
	return r.StatusCode, nil
}

type deadLink struct {
	link   string
	status int
	err    string
}

func runPause() {
	fmt.Println("Just a second, evading CAPTCHAs and 429s...")
	time.Sleep(6 * time.Second)
	fmt.Println("Sneaking past the CAPTCHAs...")
	time.Sleep(6 * time.Second)
	fmt.Println("Almost there...")
	time.Sleep(6 * time.Second)
}

// workers to sort through all of the links quicker
func worker(linkChannel <-chan string, wg *sync.WaitGroup, deadLinks *[]deadLink, mu *sync.Mutex) {
	defer wg.Done()

	for link := range linkChannel {
		status, err := checkLink(link)
		if status != 200 {
			if err == nil {
				err = fmt.Errorf("None")
			}
			d := deadLink{link: link, status: status, err: err.Error()}
			mu.Lock()
			*deadLinks = append(*deadLinks, d)
			mu.Unlock()

			fmt.Printf("[BAD] - %s ; STATUS - %d\n", link, status)
		} else {
			fmt.Printf("OK - %s\n", link)
		}
	}
}

func main() {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		fmt.Println("")
		fmt.Printf("Completed in: %dms", elapsed.Milliseconds())
		fmt.Println("")
	}()
	userUrl := askForUrl()
	checkExtDomains := askCheckExtDomain()
	parsedUrl, err := url.Parse(userUrl)
	checkNilErr(err)
	host := parsedUrl.Hostname()
	c := colly.NewCollector()
	c.IgnoreRobotsTxt = true
	linkChannel := make(chan string)
	numWorkers := 5

	if !checkExtDomains {
		c.AllowedDomains = []string{host}
	}

	var deadLinks []deadLink
	var allLinksCounter int
	var pause int
	var wg sync.WaitGroup
	var mu sync.Mutex
	var visited = make(map[string]bool)
	var visitedMu sync.Mutex

	defer func() {
		fmt.Printf("Checked %d links in total. Found %d dead links:\n", allLinksCounter, len(deadLinks))
		for _, link := range deadLinks {
			fmt.Printf("%s; status [%d]; error [%s]\n", link.link, link.status, link.err)
		}
		fmt.Println("\nDone! Press Enter to exit...")
		fmt.Scanln()
	}()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(linkChannel, &wg, &deadLinks, &mu)
	}

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting:", r.URL.String())
		r.Headers.Set("Referer", r.URL.String())
		r.Headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36")
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.5")
		r.Headers.Set("Connection", "keep-alive")
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		if !strings.HasPrefix(link, "http") {
			link = userUrl + link
		}

		if !checkExtDomains && !strings.Contains(link, host) {
			return
		}

		visitedMu.Lock()
		if visited[link] {
			visitedMu.Unlock()
			return
		}
		visited[link] = true
		visitedMu.Unlock()

		allLinksCounter++
		fmt.Printf("Found link: %s, checking status...\n", link)
		pause++
		if pause == 250 {
			runPause()
			pause = 0
		}

		linkChannel <- link
	})

	c.OnScraped(func(r *colly.Response) {
		fmt.Println("Finishing up...")
		close(linkChannel)
	})

	err = c.Visit(userUrl)
	checkNilErr(err)

	wg.Wait()
}

func checkNilErr(err error) {
	if err != nil {
		panic(err)
	}
}
