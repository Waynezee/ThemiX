import boto3
import json

# 3 nodes
regions = ["us-west-1", "us-east-1", "ap-northeast-1"]
# # 7 nodes
# regions = ["us-west-1", "us-east-1", "ap-northeast-1", "ap-southeast-2", "ap-east-1", "eu-central-1", "eu-west-1"]
# # 101 nodes
# regions = ["us-east-1", "us-west-1", "ap-northeast-1", "ap-southeast-2", "ap-east-1", "eu-central-1", "eu-west-1", "eu-west-3", "me-south-1", "sa-east-1"]

total = {}
total["nodes"] = []
clients = {}
clients["nodes"] = []
server_id = 0
client_id = 0
for region in regions:
    print("region:", region)
    ec2 = boto3.client('ec2', region_name=region)
    Tags = [{'Key': 'Name', 'Value': 'Free'}]
    # To select related machines by keyname, can use other conditions
    Filter = [
        {
            'Name': 'key-name',
            'Values': [
                'Main',
            ]
        }
    ]
    response = ec2.describe_instances(Filters=Filter)
    instances = []
    for i in range(len(response['Reservations'])):
        instances += response['Reservations'][i]['Instances']
    print(len(instances))

    # --------------------------------
    # --------nodes-----------------
    for i in range(len(instances)):
        status = instances[i]['State']['Name']
        if status != "running":
            continue
        instance = {}
        instance['Id'] = server_id
        server_id += 1
        instance['InstanceId'] = instances[i]['InstanceId']
        instance['InstanceType'] = instances[i]['InstanceType']
        instance['PublicIpAddress'] = instances[i]['PublicIpAddress']
        instance['PrivateIpAddress'] = instances[i]['PrivateIpAddress']
        instance['ServerURL'] = "http://" + \
            instances[i]['PublicIpAddress'] + ":6000/client"
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
