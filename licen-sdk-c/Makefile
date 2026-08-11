# licen-sdk-c 构建（GNU Make）
#
# 用法:
#   make            # libcurl 模式（需 libcurl-dev）
#   make no_curl    # 纯 socket 模式（零依赖）
#   make clean

CC      ?= cc
CFLAGS  ?= -O2 -Wall -Wextra -std=c11
PREFIX  ?= /usr/local

SRCS = src/licen.c src/sha256.c src/json.c
HDRS = include/licen.h

all: liblicen.a licen-demo

# libcurl 模式
liblicen.a: $(SRCS) src/http_curl.c $(HDRS)
	$(CC) $(CFLAGS) -Iinclude -c $(SRCS) src/http_curl.c
	ar rcs $@ licen.o sha256.o json.o http_curl.o
	rm -f licen.o sha256.o json.o http_curl.o

# 纯 socket 模式（零依赖）
no_curl:
	$(CC) $(CFLAGS) -DLICEN_NO_CURL=1 -Iinclude -c $(SRCS) src/http_socket.c
	ar rcs liblicen.a licen.o sha256.o json.o http_socket.o
	rm -f licen.o sha256.o json.o http_socket.o
	$(CC) $(CFLAGS) -DLICEN_NO_CURL=1 -Iinclude -o licen-demo example/demo.c liblicen.a -lpthread

licen-demo: liblicen.a
	$(CC) $(CFLAGS) -Iinclude -o $@ example/demo.c liblicen.a -lcurl -lpthread

install: liblicen.a
	install -d $(PREFIX)/include $(PREFIX)/lib
	install -m 644 $(HDRS) $(PREFIX)/include/
	install -m 644 liblicen.a $(PREFIX)/lib/

clean:
	rm -f *.o *.a licen-demo
