# MVG Schedule crawler

Scrape all scheduled trips across the Munich U-Bahn using the Overpass API to get the network topography from OpenStreetMap, and the MVV EFA Api to load departures for all line termini and plan trips to the other terminus, giving arrivals and departures for intermediate stations.

This could be extended to also crawl bus/tram routes, if you have questions about that, feel free to contact me.

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
