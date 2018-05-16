package main

import(
    "time"
    "log"
    "encoding/json"
    "io/ioutil"
    "github.com/leftshift/mvg_schedcrawl/crawler"
)

func main() {
    net := crawler.Crawl(time.Now())
    net.PrintNetwork()

    b, err := json.Marshal(net)
    if err != nil {
        log.Fatal(err)
    }
    err = ioutil.WriteFile("network.json", b, 0644)
    if err != nil {
        log.Fatal(err)
    }
}
