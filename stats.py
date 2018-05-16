import json

with open("network.json") as f:
    net = json.load(f)

print("== Number of trips ==")
for line in net['lines']:
    trips = line['trips']
    if trips is None:
        trips = []
    print(line['name'], len(trips))


print("== From-To ==")
for line in net['lines']:
    trips = line['trips']
    if trips is None:
        trips = []
    for trip in trips:
        print(line['name'], "from", trip['departures'][0]['station']['name'], "to",
              trip['departures'][-1]['station']['name'], "at",
              trip['departures'][0]['departure'])
