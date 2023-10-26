import argparse
from time import time
from typing import List
from numpy import double
from datetime import timedelta



def process_logs(num, directory, filename):
    data = []
    for i in range(num):
        ts = 0
        file = directory + '/client' + str(i) + '/' + filename
        with open(file,"r") as f:
            lines = f.readlines()
            for line in lines:
                try:
                    l = int(line)
                    data.append((ts, l))
                    ts += l
                except:
                    break
        data.sort(key=lambda x:x[0],reverse=False)
    return data

# len(data) = number of client
# data[:] is a list like [(time, latency),(time, latency),...,(time, latency)]
# latencies are in millisecond, or ms.
# base_interval: distance of two sample dots
# sample_interval: the time window of a sample dot
def analysis_data(data, interval):
    base_interval = interval
    begin_time = None
    next_begin_time = None
    result = []
    cnt = 0
    for item in data:
        timestamp = item[0]
        if begin_time is None:
            begin_time = timestamp
            next_begin_time = timestamp + base_interval
        while timestamp >= next_begin_time:
            begin_time = next_begin_time
            next_begin_time += base_interval
            result.append(cnt)
            cnt = 0
        cnt += 1
    result.append(cnt)
    
    return result 

if __name__ == '__main__':
    # read parameter, num path name
    parser = argparse.ArgumentParser(description="Sample and Analysis Script")

    parser.add_argument('-n', "--number", type=int, default="4", help="the number of client", required=True)
    parser.add_argument('-d', "--directory", type=str, default="log/client", help="directory of client logs",  required=True)
    parser.add_argument('-f', "--filename", type=str, default="latency", help="filename of log", required=True)
    parser.add_argument('-b', "--batchsize", type=int, default="1", help="client batchsize of one request ", required=True)
    parser.add_argument('-i', "--interval", type=int, default="1000", help="distance of two sample dots (ms)", required=True)

    argv = parser.parse_args()
    num = argv.number
    directory = argv.directory
    filename = argv.filename
    batchsize = argv.batchsize 
    interval = argv.interval     

    # deal with log data
    data = process_logs(num, directory, filename)

    # sample and calculate
    result = analysis_data(data, interval)

    # print result
    r = list(map(lambda x:x*batchsize, result))
    print(r)
    print("-----------------------------------------------")
