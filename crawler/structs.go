package crawler

import (
    "time"
    "errors"
    "github.com/leftshift/goefa"
)

type Departure struct {
    Line            *Line           `json:"-"`
    Station         *Station        `json:"station"`
    Destination     *Station        `json:"destination"`
    Arrival         *time.Time      `json:"arrival"`
    Departure       *time.Time      `json:"departure"`
}

// A train's journy across the network
type Trip struct {
    Departures      []*Departure    `json:"departures"`
}

// Station for multiple lines
type Station struct {
    Name            string          `json:"name"`
    Id              *int            `json:"id"`
    Lat             float64         `json:"lat"`
    Lng             float64         `json:"lng"`
    Departures      []*Departure    `json:"-"`
}

// Check if a station already has a departure at a specific time on a line with same destination
func (station *Station) HasDeparture(departureTime time.Time, line *Line, destination *Station) bool {
    _, err := station.GetDeparture(departureTime, line, destination)
    if err == nil {
        return true
    }
    return false
}

// Get Departure from station at specific time for line with destination
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

// Line, including subtrips and different directions
type Line struct {
    Name            string          `json:"name"`
    Trips           []*Trip         `json:"trips"`
    Stops           []*Station      `json:"-"`
}

// Check if a station is one of the Termini of this line
func (line *Line) IsTerminus(station *Station) bool {
    // Special case for trains without line number
    if line.Name == "U" {
        return false
    }
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
