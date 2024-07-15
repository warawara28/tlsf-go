FROM python:3.11.9-slim

# Matplotlib をインストールする。
WORKDIR /opt/app
ENV MATPLOTLIB_VERSION 3.9.1
RUN pip install matplotlib==$MATPLOTLIB_VERSION

# Matplotlib を使ったサンプルスクリプトをコピーする。
COPY plot.py bench.txt .
