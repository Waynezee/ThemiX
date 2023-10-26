import json

# change the value of ipset to the public_ips of your machines
ipset = ["18.207.160.206", "54.219.0.44", "3.112.45.205", "13.55.32.251"]


# generate nodes.json and clients.json
total = {}
total["nodes"] = []
for i in range(len(ipset)):
    instance = {}
    instance['Id'] = i

    instance['PublicIpAddress'] = ipset[i]
    instance['ServerURL'] = "http://" + ipset[i] + ":6000/client"

    total['nodes'].append(instance)

print("----- begin to load----")
file = "./nodes.json"
with open(file, "w") as f:
    json.dump(total, f)
print("----- load success ----")


for item in range(len(total['nodes'])):
    total['nodes'][item]['ServerURL'] = total['nodes'][0]['PublicIpAddress']

print("----- begin to load ----")
file = "../client/clients.json"
with open(file, "w") as f:
    json.dump(total, f)
print("----- load success ----")


# generate separate json files
cluster = []
for i in range(len(ipset)):
    cluster.append("http://" + ipset[i] + ":6000")

cluster = ','.join(cluster)
key_path = "./crypto"
pk = "./crypto"

for i in range(len(ipset)):

    file = "node%d.json" % (i,)
    data = {}
    data['id'] = i
    data['port'] = 6100  # 6000
    data['address'] = ipset[i]
    data['key_path'] = key_path
    data['pk'] = pk
    data['batchsize'] = 10000
    data['cluster'] = cluster
    with open(file, 'w') as f:
        json.dump(data, f)
