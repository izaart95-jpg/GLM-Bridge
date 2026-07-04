cmake_minimum_required(VERSION 3.14)
project(CoreVerCap3)
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)
include(FetchContent)

# ── Dynamic linking (default; no static linker flags) ─────────────────────
if(MSVC)
    set(CMAKE_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:DLL>")
endif()

# ── Threads (pthread on Linux/macOS) ──────────────────────────────────────
set(THREADS_PREFER_PTHREAD_FLAG ON)
find_package(Threads REQUIRED)

# ── nlohmann/json ─────────────────────────────────────────────────────────
FetchContent_Declare(
    nlohmann_json
    GIT_REPOSITORY https://github.com/nlohmann/json.git
    GIT_TAG        v3.11.3
)
FetchContent_MakeAvailable(nlohmann_json)

# ── ZLIB (Auto-downloaded) ────────────────────────────────────────────────
set(ZLIB_BUILD_EXAMPLES OFF CACHE BOOL "" FORCE)
FetchContent_Declare(
    zlib
    GIT_REPOSITORY https://github.com/madler/zlib.git
    GIT_TAG        v1.3.1
)
FetchContent_MakeAvailable(zlib)

# ── SQLite3 amalgamation (self-contained, no system dependency) ───────────
set(SQLITE3_DIR "${CMAKE_BINARY_DIR}/sqlite3-src")
if(NOT DEFINED SQLITE3_URL)
    set(SQLITE3_URL "https://www.sqlite.org/2024/sqlite-amalgamation-3460100.zip")
endif()
if(NOT EXISTS "${SQLITE3_DIR}/sqlite3.c")
    message(STATUS "Downloading SQLite3 amalgamation...")
    file(DOWNLOAD ${SQLITE3_URL}
         "${CMAKE_BINARY_DIR}/sqlite3.zip"
         STATUS DL_STATUS)
    list(GET DL_STATUS 0 DL_ERROR)
    if(DL_ERROR)
        message(FATAL_ERROR "Failed to download SQLite3: ${DL_STATUS}")
    endif()
    file(ARCHIVE_EXTRACT INPUT "${CMAKE_BINARY_DIR}/sqlite3.zip"
         DESTINATION "${SQLITE3_DIR}")
    file(REMOVE "${CMAKE_BINARY_DIR}/sqlite3.zip")
endif()
file(GLOB SQLITE3_SUBDIR "${SQLITE3_DIR}/sqlite-amalgamation-*")
if(NOT SQLITE3_SUBDIR)
    message(FATAL_ERROR "SQLite3 amalgamation directory not found")
endif()
add_library(sqlite3 STATIC "${SQLITE3_SUBDIR}/sqlite3.c")
target_include_directories(sqlite3 PUBLIC "${SQLITE3_SUBDIR}")
if(UNIX)
    target_link_libraries(sqlite3 PUBLIC dl)
endif()

# ── curl + OpenSSL (system, dynamically linked) ────────────────────────────
find_package(CURL REQUIRED)
find_package(OpenSSL REQUIRED)

# ── Main executable ───────────────────────────────────────────────────────
add_executable(main main.cpp)
# Expose the fetched ZLIB headers to main.cpp
target_include_directories(main PRIVATE ${zlib_SOURCE_DIR} ${zlib_BINARY_DIR})
target_link_libraries(main
    nlohmann_json::nlohmann_json
    sqlite3
    zlibstatic          # static target from FetchContent; zlib has no transitive deps so this is fine either way
    Threads::Threads
    CURL::libcurl
    OpenSSL::SSL
    OpenSSL::Crypto
)

if(UNIX AND NOT APPLE)
    target_link_libraries(main dl)
endif()

# Windows needs ws2_32/crypt32/bcrypt for Winsock and TLS/crypto support
# pulled in transitively by curl/OpenSSL.
if(WIN32)
    target_link_libraries(main ws2_32 crypt32 bcrypt)
endif()
