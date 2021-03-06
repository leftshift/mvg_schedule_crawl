package crawler

import (
    "time"
    "github.com/leftshift/goefa"
)

func getTimeAtDate(oldTime, date time.Time) (*time.Time, error) {

    loc, err := time.LoadLocation("Local")
    if err != nil {
        return nil, err
    }

    year, month, day := date.Date()
    hour, minute, second := oldTime.Clock()

    t := time.Date(year, month, day, hour, minute, second, 0, loc)
    return &t, nil
}

func filterLinesByMOT(lines []*goefa.EFAServingLine, mot int) []*goefa.EFAServingLine {
    result := make([]*goefa.EFAServingLine, 0)

    for _, line := range lines {
        if int(line.MotType) == mot {
            result = append(result, line)
        }
    }
    return result
}
