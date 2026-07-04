cmake_minimum_required(VERSION 3.10)
project(CoreVerCap3)

set(CMAKE_CXX_STANDARD 17)

include(FetchContent)
FetchContent_Declare(
    nlohmann_json
    GIT_REPOSITORY https://github.com/nlohmann/json.git
    GIT_TAG v3.11.3
)
FetchContent_MakeAvailable(nlohmann_json)

add_executable(main main.cpp)

# Link all required libraries
target_link_libraries(main
    nlohmann_json::nlohmann_json
    curl        # libcurl
    ssl         # OpenSSL
    crypto      # OpenSSL crypto
    z           # zlib
)
