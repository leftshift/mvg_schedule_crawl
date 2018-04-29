package main

import (
    "fmt"
    "time"
    "log"
//    "github.com/michiwend/goefa"
    "github.com/serjvanilla/go-overpass"
)

var query string = `
[out:json];
rel(7099055);
rel(r);
foreach(
  ._;
  rel(r)
  	->.route;
  (
  	node(r.route:"stop");
    node(r.route:"stop_exit_only");
  )->.stops;
  (
  	rel.route;
    node.stops;
  );
  out;
);`

type Departure struct {
    Arrival         *time.Time       `json:"arrival"`
    Departure       *time.Time       `json:"Departure"`
}

// Stopping position of one individual line.
type Stop struct {
    Name            string          `json:"name"`
    Departures      []*Departure    `json:"departures"`
}

// Single direction of one line
type Line struct {
    Name            string          `json:"name"`
    Stops           []*Stop         `json:"stops"`
}


type Network struct {
    Lines           []*Line         `json:"lines"`
}

func buildNetwork(result *overpass.Result) Network {
    var net Network
    for _, relation := range result.Relations {
        ref := relation.Meta.Tags["ref"]
        if ref == "" {
            continue
        }

        line := Line{Name: ref}
        net.Lines = append(net.Lines, &line)
        for _, member := range relation.Members {
            if member.Role == "stop" {
                name := member.Node.Meta.Tags["name"]
                stop := Stop{Name: name}
                line.Stops = append(line.Stops, &stop)
            }
        }
    }
    return net
}

func printNetwork(net *Network) {
    for _, line := range net.Lines {
        fmt.Println(line.Name)
        for _, stop := range line.Stops {
            fmt.Println("\t", stop.Name)
            for _, departure := range stop.Departures {
                fmt.Printf("\t\tArr:%v\t\tDep:%v", departure.Arrival, departure.Departure)
            }
        }
    }
}

func main() {
    client := overpass.New()
    result, ok := client.Query(query)
    if ok != nil {
        log.Fatal(ok)
    }
    
    net := buildNetwork(&result)
    printNetwork(&net)
}
