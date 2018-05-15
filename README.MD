# MVG Schedule crawler

Scrape all scheduled trips across the Munich U-Bahn with departure times of all stations.

This could be extended to also crawl bus/tram routes, if you have questions about that, feel free to contact me.

## Output
Currently, it outputs `network.json`, which looks like this:

```
{
    "lines": [
        {
            "name": "U2",
            "trips": [
                {
                    "departures": [
                        {
                            "station": {
                                "name": "Feldmoching",
                                "id": 1000320
                            },
                            "destination": {
                                "name": "Messestadt Ost",
                                "id": 1001260
                            },
                            "arrival": null,
                            "departure": "2018-05-15T00:04:00+10:00"
                        },
                        {
                            "station": {
                                "name": "Hasenbergl",
                                "id": null
                            },
                            "destination": {
                                "name": "Messestadt Ost",
                                "id": 1001260
                            },
                            "arrival": "2018-05-15T00:05:00+10:00",
                            "departure": "2018-05-15T00:06:00+10:00"
                        },
                        …
                    ]
                },
                …
            ]
        },
        …
    ]
}
```

## What's used
Using the Overpass API, we get the network topography from OpenStreetMap. With the MVV (EFA)[https://efa.mvv-muenchen.de/index.html] Api, we load departures for all line termini and plan trips to the other terminus, giving arrivals and departures for intermediate stations.
