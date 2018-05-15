package main

import (
    "fmt"
    "time"
    "log"
    "strings"
    "strconv"
    "errors"
    "encoding/json"
    "io/ioutil"
    "github.com/leftshift/goefa"
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
    Line            *Line           `json:"-"`
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
    Departures      []*Departure    `json:"-"`
}

func (station *Station) HasDeparture(departureTime time.Time, line *Line, destination *Station) bool {
    _, err := station.GetDeparture(departureTime, line, destination)
    if err == nil {
        return true
    }
    return false
}

func (station *Station) GetDeparture(departureTime time.Time, line *Line, destination *Station) (*Departure, error) {
    for _, dept := range station.Departures {
        if dept.Line == line &&
        dept.Destination == destination &&
        dept.Departure.Equal(departureTime) {
            return dept, nil
        }
        // optimize: start from end, return false once past specifed time
    }
    return nil, errors.New("Station doesn't have matching departure")
}

// Single direction of one line
type Line struct {
    Name            string          `json:"name"`
    Trips           []*Trip         `json:"trips"`
    Stops           []*Station      `json:"-"`
}

func (line *Line) IsTerminus(station *Station) bool {
    if station == line.Stops[0] ||
       station == line.Stops[len(line.Stops)-1] {
           return true
    }
    return false
}


type Network struct {
    Provider        *goefa.EFAProvider          `json:"-"`
    Stations        map[string]*Station         `json:"-"`
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

        if l := net.getLine(ref); l != nil {
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
    emptyLine := Line{Name:""}
    net.Lines = append(net.Lines, &emptyLine)
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

    firstTrain := time.Date(0, 0, 0, 00, 00, 00, 0, loc)
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
    fmt.Println("Got", len(departures), "departures")
    for _, dept := range departures {
        if dept.DateTime.Time.Day() != firstTrainToday.Day() {
            // It seems like this doesn't actually happen, the efa-api seems to only return departures within 24h.
            fmt.Println("Reached last departure for the day")
            return nil
        }
        if strings.HasPrefix(dept.ServingLine.Number, "U") {
            line := net.getLine(dept.ServingLine.Number)
            if ! line.IsTerminus(station) {
                if line.Name != "" {
                    // This station is not a Terminus for the given line, don't do anything about it then
                    continue
                } else {
                    // ???
                    // I honestly don't know?
                }
            }
            dTime := dept.DateTime.Time
            tmpDest := goefa.EFARouteStop{Id: dept.ServingLine.DestID} // bit of a hack, we use the ID-To-Name-Magic
            dest, err := net.getStationForEFARouteStop(&tmpDest)
            if err != nil {
                fmt.Printf("Unknown destination: %+v\n", dept)
                return err
            }

            if station.HasDeparture(*dTime, line, dest) {
                // Departure has already been added by route options offered by previous departure
                fmt.Println(station.Name, "already got departure from previous route at", dTime)
                continue
            }
            departure := Departure{Line: line, Station: station, Destination: dest, Departure: dTime}
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
    trip := Trip{
        Departures: make([]*Departure, 0),
    }
    line.Trips = append(line.Trips, &trip)

    for i, stop := range stops {
        var arr, dept *time.Time
        if len(stop.Times) == 1 {
            dept = stop.Times[0].Time
        } else {
            arr = stop.Times[0].Time
            dept = stop.Times[1].Time
        }

        if dept != nil && dept.Year() == -1 {
            // This is only used at the terminus, as the train 'never' arrives/departs.
            // No clue why they don't just use one value, but whatever.
            // Thing is, year -1 is not json-serializable, so we'll nil it instead

            dept = nil
        }
        if arr != nil && arr.Year() == -1 {
            arr = nil
        }

        intermediateStation, err := net.getStationForEFARouteStop(stop)
        if err != nil {
            return err
        }

        if i == 0 {
            // if we're on the first suggested route, this departure should already exist.
            // If we're on one of the later iterations, we still need to add it
            dept, err := intermediateStation.GetDeparture(*dept, line, destination)
            if err == nil {
                fmt.Println(intermediateStation.Name, " doesn't need departure to be added at", dept)
                trip.Departures = append(trip.Departures, dept)
                continue
            }
        }

        newDeparture := Departure{
            Line: line,
            Destination: destination,
            Station: intermediateStation,
            Arrival: arr,
            Departure: dept,
        }
        intermediateStation.Departures = append(intermediateStation.Departures, &newDeparture)
        trip.Departures = append(trip.Departures, &newDeparture)
        fmt.Printf("Added departure to intermediate %v\t%v\n", intermediateStation.Name, dept)
    }
    return nil
}

// Plans a route from the station of dept to the destination of the departure at the time of departure
// Generates departures for all intermediate stations
// Adds them to a new trip
func (net *Network) buildTrip(startDept *Departure) error {
    var numRoutes int
    fromId := startDept.Station.Id
    toId := startDept.Destination.Id
    startTime := startDept.Departure

    // Hacky and odd: For some reason, the xml api always also returns one route in the past.
    // This isn't very useful to us, so we add one minute so the first result actually starts at startTime
    // This may break at any time
    oneMin := time.Duration(time.Minute)
    t := startTime.Add(oneMin)
    startTime = &t

    if startDept.Line.Name == "" {
        // Wayy down the rabbit hole, you realize out some trains don't have numbers and I don't even 
        numRoutes = 1
    } else {
        // More hacks: If startDept doesn't span the whole line, only plan one route so we don't
        // plan only part of the later trips
        if startDept.Line.IsTerminus(startDept.Destination) {
            numRoutes = 50 // our Destination is one end of the line, so we're free to plan to our heart's content
        } else {
            fmt.Println("only planning one trip because", startDept.Destination.Name, "is not a line terminus")
            numRoutes = 1 // oh no, we can only plan one trip :(
        }
    }

    tmpFrom := goefa.EFAStop{Id: *fromId}
    tmpTo := goefa.EFAStop{Id: *toId}

    fmt.Printf("Routing from %v to %v at \t%v\n", *fromId, *toId, *startTime)
    routes, err := net.Provider.TripUsingMot(tmpFrom, tmpTo, *startTime, "dep", routeMots, numRoutes)
    if err != nil {
        return err
    }

    if len(routes) == 0 {
        return errors.New("Route from "+startDept.Station.Name+" to "+startDept.Destination.Name+ " yielded no results.")
    }

    directRoutes := make([]*goefa.EFARoute, 0)
    for _, r := range routes {
        // Only take direct routes without changing
        if startTime.Day() != r.RouteParts[0].Stops[0].Times[1].Day() {
            fmt.Println("Reached next day, breaking")
            break
        }
        if len(r.RouteParts) == 1 &&
        r.RouteParts[0].MeansOfTransport.Type == 2 && // only take U-Bahn
        r.RouteParts[0].MeansOfTransport.Shortname == startDept.Line.Name { // only take routes with the same line as the departure we're looking at
            // fmt.Printf("%+v\n", r)
            directRoutes = append(directRoutes, r)
        }
    }
    // fmt.Printf("%+v\n", route)
    for i, r := range directRoutes {
        fmt.Println("Adding intermediates for route", i)
        err = net.addIntermediateDepartures(r.RouteParts[0].Stops, startDept.Line, startDept.Destination)
        if err != nil {
            return err
        }
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
    for _, line := range net.Lines {
        // Crawl one direction
        s := line.Stops[0]
        err := net.CrawlAllDepartures(s)
        if err != nil {
            log.Fatal(err)
        }
        
        // ...and the other
        s = line.Stops[len(line.Stops)-1]
        err = net.CrawlAllDepartures(s)
        if err != nil {
            log.Fatal(err)
        }

    }

    net.printNetwork()

    b, err := json.Marshal(net)
    if err != nil {
        log.Fatal(err)
    }
    err = ioutil.WriteFile("network.json", b, 0644)
}
