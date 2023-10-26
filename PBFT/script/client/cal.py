from re import findall, search
from statistics import mean
import sys

class Logger:
    def __init__(self, n, prefix, name, size):
        self.n = n
        self.prefix = prefix
        self.name = name
        self.region_latency = {}
        self.st = []
        self.size = 0
        self.read_info()
        self.interval = 1000  # ms
        self.size = size
    
    # log pattern
    # recv: 16376 125 1686570778490 2863
    # send: 23501 125 1686570781385
    def read_info(self,):
        for i in range(self.n):
            filename = self.prefix + '/client' + str(i) + '/' + self.name
            txs = 0
            latency = 0
            with open(filename, 'r') as f:
                log = f.read()
                tmp = findall(r'recv: (\d+) (\d+) (\d+) (\d+)', log)
                tmp = [(int(id), int(size), int(st), int(t)/1000) for id, size, st, t in tmp]
                self.region_latency[i] = [item[3] for item in tmp]
                self.st += [item[2] for item in tmp]
                # In a test, self.size is fixed, every request has same size
                # self.size = tmp[0][1]
    
    def average_latency(self):
        lat = []
        for i in range(self.n):
            lat += self.region_latency[i]
        return mean(lat) if lat else 0

    def p95_latency(self):
        lat = []
        for i in range(self.n):
            lat += self.region_latency[i]
        lat = sorted(lat)
        return lat[int(0.95 * len(lat))] if lat else 0


    def region_average_latency(self, k):
        ret = []
        lat = []
        for i in range(self.n):
            lat += self.region_latency[i]
            if (i + 1) % k == 0:
                ret += [mean(lat)] if lat else [0]
                lat = []  
        return ret

    def region_p95_latency(self, k):
        ret = []
        lat = []
        for i in range(self.n):
            lat += self.region_latency[i]
            if (i + 1) % k == 0:
                lat = sorted(lat)
                ret += [lat[int(0.95 * len(lat))] if lat else 0]
                lat = []
            
        return ret

    def total_tps(self):
        commits = 0
        for i in range(self.n):
            commits += len(self.region_latency[i])
        return commits * self.size
    
    def region_tps(self, k):
        ret = []
        commits = 0
        for i in range(self.n):
            commits = len(self.region_latency[i])
            if (i + 1) % k == 0:
                ret += [commits * self.size]
                commits = 0
            
        return ret
    
    def tps_series(self):
        self.st = sorted(self.st)
        next_begin_time = None
        result = []
        cnt = 0
        for st in self.st:
            if next_begin_time is None:
                next_begin_time = int(st) + self.interval
            while st > next_begin_time:
                next_begin_time += self.interval
                result.append(cnt)
                cnt = 0
            cnt += self.size
            # print(cnt)
        result.append(cnt)

        return result


if __name__ == "__main__":
    if len(sys.argv) != 6:
        print("usage: python3 cal.py [number of clients] [request size] [path of log] [name of logfile] [test time] ")
    total_nodes = int(sys.argv[1])
    size = int(sys.argv[2])
    prefix = str(sys.argv[3])
    name = str(sys.argv[4])
    duration = int(sys.argv[5])
    d = Logger(total_nodes, prefix, name, size)
    print("total_tps: ", d.total_tps()/duration)
    print("average_latency: ", d.average_latency())
    print("p95_latency: ", d.p95_latency())
    # print("region_average_latency: ", d.region_average_latency(1))
    # print("region_p95_latency: ", d.region_p95_latency(1))
    # print("region_tps: ", d.region_tps(1))
    # print(d.tps_series())

    

