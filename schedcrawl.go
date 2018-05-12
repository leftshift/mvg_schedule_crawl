package main

import (
    "fmt"
    "time"
    "log"
    "strings"
    "strconv"
//    "sort"
    "errors"
    "github.com/michiwend/goefa"
    "github.com/serjvanilla/go-overpass"
    "github.com/renstrom/fuzzysearch/fuzzy"
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

var routeMots []goefa.EFAMotType = []goefa.EFAMotType{2}

// used to match names from openStreetMap with names from EFA
var stationNameSanitizer = strings.NewReplacer(
    "-", "",
    ".", "",
    "(", "",
    ")", "",
    " ", "")

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

func (station *Station) HasDeparture(departureTime time.Time, line *Line, destination *Station) bool {
    for _, dept := range station.Departures {
        if dept.Line == line &&
        dept.Destination == destination &&
        dept.Departure.Equal(departureTime) {
            return true
        }
        // optimize: start from end, return false once past specifed time
    }
    return false
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
            if member.Role == "stop" || member.Role == "stop_exit_only" {
                name := member.Node.Meta.Tags["name"]

                var station *Station

                if s, ok := net.Stations[name]; ok {
                    station = s
                } else {
                    s := Station{Name: name, Departures: make([]*Departure, 0)}
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

// Get the station for an EFAstop, pulling out all the stops (no pun intended)
// First see if a station of that name exists, if not, check if the id exists
// If both fail, do a stopfinder request on the ID
// If that still yields an unknown name, do a fuzzy search on known stations
func (net *Network) getStationForEFARouteStop(stop *goefa.EFARouteStop) (*Station, error) {
    // See if stop name matches name in network
    station, ok := net.Stations[stop.Name]
    // This sometimes fails because intermediate Stops in the RoutePart
    // sometimes have abbreviated names.
    if ok {
        return station, nil
    }

    // See if stop ID matches ID in network
    for _, station := range net.Stations {
        if station.Id != nil && *station.Id == stop.Id {
            return station, nil
        }
    }

    // See if name returned by searching for stop ID matches name in network
    idft, stops, err := net.Provider.FindStop(strconv.Itoa(stop.Id))
    if !idft {
        return nil, errors.New("Station " + stop.Name + " wasn't uniquely identified despite using ID " + strconv.Itoa(stop.Id))
    }
    if err != nil {
        return nil, err
    }
    name := stops[0].Name

    station, ok = net.Stations[name]
    if ok {
        station.Id = &stop.Id
        return station, nil
    }

    // See if stop name fuzzy-matches name in Network
    stationNames := make([]string, 0)
    stationPtrs := make([]*Station, 0)
    for _, station := range net.Stations {
        sanitizedName := stationNameSanitizer.Replace(station.Name)
        stationNames = append(stationNames, sanitizedName)
        stationPtrs = append(stationPtrs, station)
    }
    fmt.Printf("fuzzy searching for %v in %v\n", stop.Name, stationNames)
    sanitizedStopName := stationNameSanitizer.Replace(stop.Name)
    matches := fuzzy.Find(sanitizedStopName, stationNames)
    if len(matches) == 0 {
        return nil, errors.New(stop.Name + " not fuzzy-found in network")
    }
    if len(matches) > 1 {
        fmt.Println(matches)
        return nil, errors.New("Fuzzy search matched multiple stations")
    }
    for i, name := range stationNames {
        if matches[0] == name {
            station = stationPtrs[i]
        }
    }
    station.Id = &stop.Id
    return station, nil
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

    fmt.Printf("Crawling %v starting at %v\n", station.Name, firstTrainToday)

    ident, stops, err := net.Provider.FindStop(station.Name)
    if err != nil {
        return err
    }
    if ident != true {
        log.Printf("Stop ", station.Name, " was not uniquely identified!")
    }
    stop := stops[0]

    stopLines, err := stop.Lines()
    if err != nil {
        return err
    }
    subwayLines := filterLinesByMOT(stopLines, 2)

    station.Id = &stop.Id

    departures, err := stop.DeparturesForLines(*firstTrainToday, 500, subwayLines)
    if err != nil {
        return err
    }
    for _, dept := range departures {
        if strings.HasPrefix(dept.ServingLine.Number, "U") {
            line := net.getLine(dept.ServingLine.Number)
            dTime := dept.DateTime.Time
            tmpDest := Station{Id: &dept.ServingLine.DestID}
            departure := Departure{Line: line, Station: station, Destination: &tmpDest, Departure: dTime}
            station.Departures = append(station.Departures, &departure)
            // fmt.Println("Added dept to:", station)

            if err := net.buildTrip(&departure); err != nil {
                return err
            }
        }
        //fmt.Printf("%+v\n", dept)
    }

    return nil
}

func (net *Network) addIntermediateDepartures(stops []*goefa.EFARouteStop, line *Line, destination *Station) error {
    for i, stop := range stops {
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

        intermediateStation, err := net.getStationForEFARouteStop(stop)
        if err != nil {
            return err
        }
        newDeparture := Departure{
            Line: line,
            Destination: destination,
            Station: intermediateStation,
            Arrival: arr,
            Departure: dept,
        }
        intermediateStation.Departures = append(intermediateStation.Departures, &newDeparture)
        fmt.Printf("Added departure to intermediate %v\n", intermediateStation.Name)
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

    fmt.Printf("Routing from %v to %v\n", fromId, toId)
    routes, err := net.Provider.TripUsingMot(tmpFrom, tmpTo, *startTime, "dep", routeMots)
    if err != nil {
        return err
    }

    directRoutes := make([]*goefa.EFARoute, 0)
    for _, r := range routes {
        // Only take direct routes without changing
        if len(r.RouteParts) == 1 &&
        r.RouteParts[0].MeansOfTransport.Type == 2 && // only take U-Bahn
        r.RouteParts[0].MeansOfTransport.Shortname == startDept.Line.Name { // only take routes with the same line as the departure we're looking at
            fmt.Printf("%+v\n", r)
            directRoutes = append(directRoutes, r)
        }
    }
    // fmt.Printf("%+v\n", route)
    for _, r = range directRoutes {
        err = net.addIntermediateDepartures(r.RouteParts[0].Stops, startDept.Line, startDept.Destination)
        if err != nil {
            return err
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

    //l := net.Lines[0]
    //s := l.Stops[0]
    s := net.Stations["Mangfallplatz"]
    err = net.CrawlAllDepartures(s)
    if err != nil {
        log.Fatal(err)
    }

    net.printNetwork()
}
