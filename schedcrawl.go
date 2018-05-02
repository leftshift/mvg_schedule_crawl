package main

import (
    "fmt"
    "time"
    "log"
    "strings"
    "errors"
    "github.com/michiwend/goefa"
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
    Line            *Line
    Station         *Station        `json:"station"`
    Destination     *Station        `json:"destination"`
    Arrival         *time.Time      `json:"arrival"`
    Departure       *time.Time      `json:"departure"`
}

type Trip struct {
    Departures      []*Departure    `json:"departures"`
}

// Stopping position of one individual line.
type Station struct {
    Name            string          `json:"name"`
    Id              *int            `json:"id"`
    Departures      []*Departure    `json:"departures"`
}

// Single direction of one line
type Line struct {
    Name            string          `json:"name"`
    Trips           []*Trip         `json:"trips"`
    Stops           []*Station      `json:"stops"`
}


type Network struct {
    Provider        *goefa.EFAProvider
    Stations        map[string]*Station      `json:"stations"`
    Lines           []*Line         `json:"lines"`
}


func buildNetwork(result *overpass.Result) Network {
    var net Network
    net.Stations = make(map[string]*Station)
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

                var station *Station

                if s, ok := net.Stations[name]; ok {
                    station = s
                } else {
                    s := Station{Name: name}
                    net.Stations[name] = &s
                    station = &s
                }
                line.Stops = append(line.Stops, station)
            }
        }
    }
    return net
}

func (net *Network) getLine(name string) *Line {
    for _, line := range net.Lines {
        if line.Name == name {
            return line
        }
    }
    return nil
}

func (net *Network) printNetwork() {
    for _, line := range net.Lines {
        fmt.Println(line.Name)
        for _, stop := range line.Stops {
            fmt.Println("\t", stop.Name)
            for _, departure := range stop.Departures {
                fmt.Printf("\t\tArr:%v\t\tDep:%v\n", departure.Arrival, departure.Departure)
            }
        }
    }
}

func (net *Network) CrawlAllDepartures(station *Station) error {
    loc, err := time.LoadLocation("Local")
    if err != nil {
        return err
    }

    firstTrain := time.Date(0, 0, 0, 03, 00, 00, 0, loc)
    firstTrainToday, err := getTimeToday(firstTrain)

    if err != nil {
        return err
    }

    fmt.Println(firstTrainToday, station.Name)

    ident, stops, err := net.Provider.FindStop(station.Name)
    if err != nil {
        return err
    }
    if ident != true {
        log.Printf("Stop ", station.Name, " was not uniquely identified!")
    }
    stop := stops[0]

    station.Id = &stop.Id

    departures, err := stop.Departures(*firstTrainToday, 500)
    if err != nil {
        return err
    }
    fmt.Println(len(departures))
    for _, dept := range departures {
        if strings.HasPrefix(dept.ServingLine.Number, "U") {
            line := net.getLine(dept.ServingLine.Number)
            dTime := dept.DateTime.Time
            tmpDest := Station{Id: &dept.ServingLine.DestID}
            departure := Departure{Line: line, Station: station, Destination: &tmpDest, Departure: dTime}
            station.Departures = append(station.Departures, &departure)
            fmt.Println("Added dept to:", station)

            if err := net.buildTrip(&departure); err != nil {
                return err
            }
        }
        fmt.Printf("%+v\n", dept)
    }

    return nil
}

// Plans a route from the station of dept to the destination of the departure at the time of departure
// Generates departures for all intermediate stations
// Adds them to a new trip
func (net *Network) buildTrip(startDept *Departure) error {
    fromId := startDept.Station.Id
    toId := startDept.Destination.Id
    startTime := startDept.Departure

    tmpFrom := goefa.EFAStop{Id: *fromId}
    tmpTo := goefa.EFAStop{Id: *toId}

    routes, err := net.Provider.Trip(tmpFrom, tmpTo, *startTime, "dep")
    if err != nil {
        return err
    }

    fmt.Printf("%+v\n", routes)
    var route *goefa.EFARoute
    for _, r := range routes {
        // Only take direct routes without changing
        if len(r.RouteParts) == 1 &&
        r.RouteParts[0].MeansOfTransport.Type == 2{
            route = r
        }
    }
    fmt.Printf("%+v\n", route)

    for i, stop := range route.RouteParts[0].Stops {
        if i == 0 {
            // First station already has departure
            continue
        }
        var arr, dept *time.Time
        if len(stop.Times) == 1 {
            dept = stop.Times[0].Time
        } else {
            arr = stop.Times[0].Time
            dept = stop.Times[1].Time
        }

        s, ok := net.Stations[stop.Name]
        if !ok {
            return errors.New("Station name " + stop.Name + " not found in network")
        }
        newDeparture := Departure{
            Line: startDept.Line,
            Destination: startDept.Destination,
            Station: s,
            Arrival: arr,
            Departure: dept,
        }
        s.Departures = append(s.Departures, &newDeparture)
    }
    return nil
}

func main() {
    efaProv, err := goefa.ProviderFromJson("mvv")
    if err != nil {
        log.Fatal(err)
    }

    client := overpass.New()
    result, ok := client.Query(query)
    if ok != nil {
        log.Fatal(ok)
    }
    
    net := buildNetwork(&result)
    net.Provider = efaProv

    l := net.Lines[0]
    s := l.Stops[0]
    err = net.CrawlAllDepartures(s)
    if err != nil {
        log.Fatal(err)
    }

    net.printNetwork()
}
