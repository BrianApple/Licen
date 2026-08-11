cmake_minimum_required(VERSION 3.10)
project(licen-sdk-c C)

set(CMAKE_C_STANDARD 11)

option(LICEN_NO_CURL "使用纯 POSIX socket 实现（零依赖）" OFF)

set(LICEN_SOURCES
    src/licen.c
    src/sha256.c
    src/json.c
)

if(LICEN_NO_CURL)
    message(STATUS "licen-sdk-c: 纯 socket 模式（零依赖）")
    list(APPEND LICEN_SOURCES src/http_socket.c)
    add_compile_definitions(LICEN_NO_CURL=1)
else()
    message(STATUS "licen-sdk-c: libcurl 模式")
    list(APPEND LICEN_SOURCES src/http_curl.c)
    find_package(CURL REQUIRED)
endif()

# 静态库
add_library(licen STATIC ${LICEN_SOURCES})
target_include_directories(licen PUBLIC ${CMAKE_CURRENT_SOURCE_DIR}/include)
if(NOT LICEN_NO_CURL)
    target_link_libraries(licen PUBLIC CURL::libcurl)
endif()
find_package(Threads REQUIRED)
target_link_libraries(licen PUBLIC Threads::Threads)

# 示例
add_executable(licen-demo example/demo.c)
target_link_libraries(licen-demo PRIVATE licen)
