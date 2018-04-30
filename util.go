package main

import (
    "time"
)

func getTimeToday(oldTime time.Time) (*time.Time, error) {
    today := time.Now()

    loc, err := time.LoadLocation("Local")
    if err != nil {
        return nil, err
    }

    year, month, day := today.Date()
    hour, minute, second := oldTime.Clock()

    t := time.Date(year, month, day, hour, minute, second, 0, loc)
    return &t, nil
}
