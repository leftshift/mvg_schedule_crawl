package main

import (
    "time"
    "log"
//    "io/ioutil"
    "github.com/artonge/go-gtfs"
    "github.com/leftshift/mvg_schedcrawl/crawler"
)

func addStations(g *gtfs.GTFS, stations []*crawler.Station) {
    for _, station := range stations {
        s = gtfs.Stop{
            ID:         station.Id,
            Name:       station.Name,
            
        }
    }
}

func main() {
    net := crawler.Crawl(time.Now())
    net.PrintNetwork()
    g = gtfs.GTFS{}
}
