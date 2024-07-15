import sys
import re
from matplotlib import pyplot as plt

def parse_benchmark_data():
    data = {}
    with open('bench.txt', 'r') as file:
    # Read the file line by line
        for line in file:
            print("line: ", line)
            match = re.search(r'Benchmark(\w+)/testBytes=(\d+)\s+\d+\s+([\d.]+) ns/op', line)
            if match:
                print("match")
                benchmark, test_bytes, ns_op = match.groups()
                if benchmark not in data:
                    data[benchmark] = {'x': [], 'y': []}
                data[benchmark]['x'].append(int(test_bytes))
                data[benchmark]['y'].append(float(ns_op))
    return data

if __name__ == '__main__':
    data = parse_benchmark_data()
    print(data)
    
    print('Plotting...')
    plt.figure(figsize=(12, 8))
    for benchmark, values in data.items():
        plt.plot(values['x'], values['y'], marker='o', label=benchmark)
    
    plt.xscale('log')
    plt.yscale('log')
    plt.xlabel('Test Bytes')
    plt.ylabel('ns/op')
    plt.title('Benchmark Results')
    plt.legend()
    plt.grid(True)
    plt.savefig('benchmark_results.png')
