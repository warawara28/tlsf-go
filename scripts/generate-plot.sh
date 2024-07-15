#!/bin/sh -eu

BENCHMARK_SCRIPT="go test -bench  . -benchmem -benchtime=1000x -cpu=1 -tags goexperiment.arenas"
SCRIPT_DIR=$(cd $(dirname $0); pwd)
BENCHMARK_TEMP_FILE="bench.txt"
FILENAME="benchmark_results.png"

# Run the benchmark
eval "$BENCHMARK_SCRIPT" > $SCRIPT_DIR/$BENCHMARK_TEMP_FILE

# Plot the results
docker build -t plotter $SCRIPT_DIR/.
docker run --rm -v $SCRIPT_DIR:/opt/data \
  -w /opt/data \
  plotter python3 /opt/app/plot.py 

mv $SCRIPT_DIR/$FILENAME $SCRIPT_DIR/../
