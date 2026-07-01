#include <openssl/hmac.h>
#include <openssl/evp.h>
#include <openssl/bio.h>
#include <openssl/buffer.h>

#include <curl/curl.h>
#include <nlohmann/json.hpp>
#include <zlib.h>
#include <sqlite3.h>
#include <iostream>
#include <algorithm>
#include <cstdint>
#include <chrono>
#include <cstring>
#include <ctime>
#include <cctype>
#include <iomanip>
#include <map>
#include <random>
#include <set>
#include <sstream>
#include <string>
#include <vector>
#include <mutex>
#include <atomic>
#include <csignal>
#include <fstream>

// Cross-platform named pipe headers
#ifdef _WIN32
    #include <windows.h>
    typedef HANDLE PipeHandle;
    #define PIPE_INVALID INVALID_HANDLE_VALUE
#else
    #include <sys/stat.h>
    #include <sys/types.h>
    #include <fcntl.h>
    #include <unistd.h>
    #include <errno.h>
    #include <cstdio>
    typedef int PipeHandle;
    #define PIPE_INVALID (-1)
#endif

using json = nlohmann::json;

// ============================================================================
// Configuration
// ============================================================================
static const std::string ACCESS_KEY  = "LTAI5tSEBwYMwVKAQGpxmvTd";
static const std::string SECRET_KEY  = "YSKfst7GaVkXwZYvVihJsKF9r89koz";
static const std::string SCENE_ID    = "didk33e0";
static std::string       DB_PATH     = "tokens.sqlite";
static const std::string PIPE_NAME   = "captcha_pipe";
static const int  MAX_TOKEN_RETRIES  = 20;

static std::atomic<bool> g_running{true};
static std::mutex        g_db_mutex;
static std::atomic<bool> g_verbose{false};

// ============================================================================
// Error-only logging — silent unless --verbose is passed
// ============================================================================
static void log_error(const std::string& msg) {
    if (!g_verbose) return;
    auto now   = std::chrono::system_clock::now();
    auto now_t = std::chrono::system_clock::to_time_t(now);
    std::tm tm_utc{};
#ifdef _WIN32
    gmtime_s(&tm_utc, &now_t);
#else
    gmtime_r(&now_t, &tm_utc);
#endif
    char buf[32];
    std::strftime(buf, sizeof(buf), "%Y-%m-%dT%H:%M:%SZ", &tm_utc);
    std::cerr << "[" << buf << "] ERROR: " << msg << std::endl;
}

// ============================================================================
// Utility: URL encoding
// ============================================================================
std::string url_encode(const std::string& s, const std::string& safe = "") {
    static const char hex[] = "0123456789ABCDEF";
    std::set<char> safe_set(safe.begin(), safe.end());
    std::string out;
    out.reserve(s.size() * 3);
    for (unsigned char c : s) {
        if (std::isalnum(c) || c == '-' || c == '_' || c == '.' || c == '~'
            || safe_set.count(static_cast<char>(c))) {
            out.push_back(static_cast<char>(c));
        } else {
            out.push_back('%');
            out.push_back(hex[c >> 4]);
            out.push_back(hex[c & 0x0F]);
        }
    }
    return out;
}

// ============================================================================
// Utility: Base64 encode
// ============================================================================
std::string base64_encode(const std::string& data) {
    BIO* b64 = BIO_new(BIO_f_base64());
    BIO* mem = BIO_new(BIO_s_mem());
    b64 = BIO_push(b64, mem);
    BIO_set_flags(b64, BIO_FLAGS_BASE64_NO_NL);
    BIO_write(b64, data.data(), static_cast<int>(data.size()));
    BIO_flush(b64);
    BUF_MEM* bptr = nullptr;
    BIO_get_mem_ptr(b64, &bptr);
    std::string result(bptr->data, bptr->length);
    BIO_free_all(b64);
    return result;
}

std::string base64_encode(const std::vector<uint8_t>& data) {
    return base64_encode(std::string(data.begin(), data.end()));
}

// ============================================================================
// HMAC-SHA1
// ============================================================================
std::string hmac_sha1(const std::string& key, const std::string& msg) {
    unsigned char digest[EVP_MAX_MD_SIZE];
    unsigned int digest_len = 0;
    HMAC(EVP_sha1(),
         key.data(), static_cast<int>(key.size()),
         reinterpret_cast<const unsigned char*>(msg.data()),
         msg.size(),
         digest, &digest_len);
    return std::string(reinterpret_cast<char*>(digest), digest_len);
}

// ============================================================================
// UUID v4
// ============================================================================
std::string generate_uuid() {
    static std::random_device rd;
    static std::mt19937_64 gen(rd());
    std::uniform_int_distribution<uint64_t> dis(0, UINT64_MAX);
    uint64_t a = dis(gen);
    uint64_t b = dis(gen);
    a = (a & 0x0FFFFFFFFFFFFFFFULL) | 0x4000000000000000ULL;
    b = (b & 0x3FFFFFFFFFFFFFFFULL) | 0x8000000000000000ULL;
    std::ostringstream oss;
    oss << std::hex << std::setfill('0');
    oss << std::setw(8) << (a >> 32) << "-";
    oss << std::setw(4) << ((a >> 16) & 0xFFFF) << "-";
    oss << std::setw(4) << (a & 0xFFFF) << "-";
    oss << std::setw(4) << (b >> 48) << "-";
    oss << std::setw(12) << (b & 0x0000FFFFFFFFFFFFULL);
    return oss.str();
}

// ============================================================================
// Timestamp helpers
// ============================================================================
std::string get_timestamp_utc() {
    auto now   = std::chrono::system_clock::now();
    auto now_t = std::chrono::system_clock::to_time_t(now);
    std::tm tm_utc{};
#ifdef _WIN32
    gmtime_s(&tm_utc, &now_t);
#else
    gmtime_r(&now_t, &tm_utc);
#endif
    char buf[32];
    std::strftime(buf, sizeof(buf), "%Y-%m-%dT%H:%M:%SZ", &tm_utc);
    return std::string(buf);
}

int64_t current_time_millis() {
    auto now = std::chrono::system_clock::now();
    return static_cast<int64_t>(
        std::chrono::duration_cast<std::chrono::milliseconds>(
            now.time_since_epoch()).count());
}

// ============================================================================
// Aliyun signature
// ============================================================================
std::string generate_signature(const std::map<std::string, std::string>& params,
                               const std::string& secret_key) {
    std::vector<std::pair<std::string, std::string>> sorted(params.begin(), params.end());
    std::sort(sorted.begin(), sorted.end(),
              [](const auto& a, const auto& b) { return a.first < b.first; });

    std::string canonical;
    for (size_t i = 0; i < sorted.size(); ++i) {
        if (i > 0) canonical += "&";
        canonical += url_encode(sorted[i].first);
        canonical += "=";
        canonical += url_encode(sorted[i].second);
    }
    std::string string_to_sign = "POST&" + url_encode("/") + "&" + url_encode(canonical);
    std::string signing_key = secret_key + "&";
    return base64_encode(hmac_sha1(signing_key, string_to_sign));
}

// ============================================================================
// libcurl write callback
// ============================================================================
static size_t write_callback(void* contents, size_t size, size_t nmemb, void* userp) {
    size_t total = size * nmemb;
    static_cast<std::string*>(userp)->append(static_cast<char*>(contents), total);
    return total;
}

// ============================================================================
// HTTP POST
// ============================================================================
std::string http_post(const std::string& url,
                      const std::string& body,
                      const std::vector<std::pair<std::string, std::string>>&
                          extra_headers = {}) {
    CURL* curl = curl_easy_init();
    if (!curl) throw std::runtime_error("Failed to init curl");

    std::string response_data;
    struct curl_slist* headers = nullptr;
    headers = curl_slist_append(headers,
        "Content-Type: application/x-www-form-urlencoded; charset=UTF-8");
    for (const auto& h : extra_headers)
        headers = curl_slist_append(headers, (h.first + ": " + h.second).c_str());

    curl_easy_setopt(curl, CURLOPT_URL, url.c_str());
    curl_easy_setopt(curl, CURLOPT_POST, 1L);
    curl_easy_setopt(curl, CURLOPT_POSTFIELDS, body.c_str());
    curl_easy_setopt(curl, CURLOPT_POSTFIELDSIZE, (long)body.size());
    curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_callback);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &response_data);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, 30L);
    curl_easy_setopt(curl, CURLOPT_SSL_VERIFYPEER, 1L);

    CURLcode res = curl_easy_perform(curl);
    curl_slist_free_all(headers);
    curl_easy_cleanup(curl);

    if (res != CURLE_OK)
        throw std::runtime_error(std::string("curl error: ") + curl_easy_strerror(res));
    return response_data;
}

// ============================================================================
// SQLite token management — read one, delete after use
// ============================================================================
bool get_next_token(std::string& token) {
    std::lock_guard<std::mutex> lk(g_db_mutex);

    {
        std::ifstream probe(DB_PATH, std::ios::binary);
        if (!probe.good()) {
            log_error("Database file not found: " + DB_PATH);
            return false;
        }
    }

    sqlite3* db = nullptr;
    if (sqlite3_open(DB_PATH.c_str(), &db) != SQLITE_OK) {
        log_error("Cannot open DB '" + DB_PATH + "': " +
                   (db ? sqlite3_errmsg(db) : "unknown error"));
        sqlite3_close(db);
        return false;
    }
    sqlite3_stmt* stmt = nullptr;
    bool found = false;
    if (sqlite3_prepare_v2(db, "SELECT token FROM tokens ORDER BY id LIMIT 1;",
                           -1, &stmt, nullptr) != SQLITE_OK) {
        log_error("Failed to prepare token SELECT: " + std::string(sqlite3_errmsg(db)));
    } else if (sqlite3_step(stmt) == SQLITE_ROW) {
        const unsigned char* t = sqlite3_column_text(stmt, 0);
        if (t) { token = reinterpret_cast<const char*>(t); found = true; }
    } else {
        log_error("No device tokens available in table 'tokens'");
    }
    sqlite3_finalize(stmt);
    sqlite3_close(db);
    return found;
}

void remove_token(const std::string& token) {
    std::lock_guard<std::mutex> lk(g_db_mutex);
    sqlite3* db = nullptr;
    if (sqlite3_open(DB_PATH.c_str(), &db) != SQLITE_OK) {
        log_error("Cannot open DB '" + DB_PATH + "' to remove token: " +
                   (db ? sqlite3_errmsg(db) : "unknown error"));
        sqlite3_close(db);
        return;
    }
    sqlite3_stmt* stmt = nullptr;
    if (sqlite3_prepare_v2(db, "DELETE FROM tokens WHERE token = ?;",
                           -1, &stmt, nullptr) != SQLITE_OK) {
        log_error("Failed to prepare token DELETE: " + std::string(sqlite3_errmsg(db)));
    } else {
        sqlite3_bind_text(stmt, 1, token.c_str(), -1, SQLITE_TRANSIENT);
        if (sqlite3_step(stmt) != SQLITE_DONE) {
            log_error("Failed to delete consumed token: " + std::string(sqlite3_errmsg(db)));
        }
    }
    sqlite3_finalize(stmt);
    sqlite3_close(db);
}

// ============================================================================
// PART 1: InitCaptchaV3 (silent)
// ============================================================================
std::string init_captcha(const std::string& access_key,
                         const std::string& secret_key,
                         const std::string& scene_id) {
    std::map<std::string, std::string> params = {
        {"AccessKeyId",      access_key},
        {"Action",           "InitCaptchaV3"},
        {"Format",           "JSON"},
        {"Language",         "en"},
        {"Mode",             "popup"},
        {"SceneId",          scene_id},
        {"SignatureMethod",  "HMAC-SHA1"},
        {"SignatureNonce",   generate_uuid()},
        {"SignatureVersion", "1.0"},
        {"Timestamp",        get_timestamp_utc()},
        {"UpLang",           "true"},
        {"Version",          "2023-03-05"},
    };
    std::string signature = generate_signature(params, secret_key);
    params["Signature"] = signature;

    std::string body;
    bool first = true;
    for (const auto& kv : params) {
        if (!first) body += "&";
        first = false;
        body += url_encode(kv.first) + "=" + url_encode(kv.second);
    }
    std::string response = http_post(
        "https://no8xfe.captcha-open-southeast.aliyuncs.com/", body);
    return json::parse(response)["CertifyId"].get<std::string>();
}

// ============================================================================
// PART 2: Generate arg
// ============================================================================
std::string generate_arg(const std::string& certify_id,
                         const std::string& constant = "4xrihv8zb8tf1mfj") {
    std::string encoded = url_encode(certify_id);
    std::string o;
    for (size_t i = 0; i < encoded.size();) {
        if (encoded[i] == '%' && i + 2 < encoded.size()) {
            o.push_back(static_cast<char>(std::stoi(encoded.substr(i+1, 2), nullptr, 16)));
            i += 3;
        } else {
            o.push_back(encoded[i++]);
        }
    }
    std::string n = constant;
    std::vector<int> r = {
        32,50,10,51,6,44,37,16,46,11,62,19,43,25,23,30,
        60,33,53,34,7,26,12,48,5,2,20,4,61,13,47,49,
        18,29,27,22,1,17,39,56,41,38,55,31,15,58,52,40,
        8,57,45,35,59,36,42,54,63,3,24,28,14,9,0,21
    };
    int rlen = static_cast<int>(r.size());
    int i = 0, j = 0;
    while (i < rlen) {
        j = (((i + j + r[i] + r[j]) >> 1) + n[i % n.size()]) & (rlen - 1);
        if (i != j) { r[i] ^= r[j]; r[j] ^= r[i]; r[i] ^= r[j]; }
        i += 1;
    }
    std::string t;
    int e = 0, a = 0;
    for (size_t idx = 0; idx < o.size(); ++idx) {
        a = ((e ^ a) + (r[e] ^ r[a])) & (rlen - 1);
        if (e != a) { r[e] ^= r[a]; r[a] ^= r[e]; r[e] ^= r[a]; }
        int m = static_cast<unsigned char>(o[idx]);
        m = m + e + r[e] - a - r[a];
        m = m ^ (r[e] + r[a]);
        m = m ^ r[(r[e] + r[a]) & (rlen - 1)];
        m = m & 255;
        t.push_back(static_cast<char>(m));
        e = (e + 1) & (rlen - 1);
    }
    return base64_encode(t);
}

// ============================================================================
// PART 4: ali_hash
// ============================================================================
std::string ali_hash(const std::string& input_str, const std::string& salt_str) {
    std::string o = input_str;
    std::string r = salt_str;
    int a_len = static_cast<int>(o.size());
    int m     = static_cast<int>(r.size());

    std::vector<int> e;
    for (int i = 0; i < 16; ++i) e.push_back((i << 4) + (i % 16));
    int f = static_cast<int>(e.size());

    int i = 0, j = 0;
    while (i < f) {
        j = (((i + j + e[i] + e[j]) >> 1) + r[i % m]) & (f - 1);
        std::swap(e[i], e[j]);
        i += 1;
    }
    int idx = 0, p = 0, q = 0;
    while (idx < a_len) {
        q = ((p ^ q) + (e[p] ^ e[q])) & (f - 1);
        std::swap(e[p], e[q]);
        int C = static_cast<unsigned char>(o[idx]);
        C = (C + p + q) ^ e[p] ^ e[q];
        C = C & 255;
        e[p] = C;
        p = (p + 1) & (f - 1);
        idx += 1;
    }
    for (int step = 0; step < 2 * f; ++step) {
        int pos = step % f;
        if (pos != 0) e[pos] ^= e[pos - 1];
        else          e[0] ^= e[f - 1];
    }
    std::ostringstream result;
    for (int b : e)
        result << std::hex << std::setw(2) << std::setfill('0') << (b & 0xFF);
    return result.str();
}

// ============================================================================
// PART 7: encrypt
// ============================================================================
std::string encrypt(const std::vector<uint8_t>& plaintext_bytes) {
    std::string o(plaintext_bytes.begin(), plaintext_bytes.end());
    std::string n = "3e627e1b4c63f913";
    std::vector<int> r = {
        32,50,10,51,6,44,37,16,46,11,62,19,43,25,23,30,
        60,33,53,34,7,26,12,48,5,2,20,4,61,13,47,49,
        18,29,27,22,1,17,39,56,41,38,55,31,15,58,52,40,
        8,57,45,35,59,36,42,54,63,3,24,28,14,9,0,21
    };
    int rlen = static_cast<int>(r.size());
    int o_ksa = 0, t_ksa = 0;
    while (o_ksa < rlen) {
        t_ksa = (((o_ksa + t_ksa + r[o_ksa] + r[t_ksa]) >> 1)
                 + n[o_ksa % n.size()]) & (rlen - 1);
        if (o_ksa != t_ksa) {
            r[o_ksa] ^= r[t_ksa]; r[t_ksa] ^= r[o_ksa]; r[o_ksa] ^= r[t_ksa];
        }
        o_ksa += 1;
    }
    std::string t;
    int n_prga = 0, e = 0, a = 0;
    while (n_prga < static_cast<int>(o.size())) {
        a = ((e ^ a) + (r[e] ^ r[a])) & (rlen - 1);
        if (e != a) { r[e] ^= r[a]; r[a] ^= r[e]; r[e] ^= r[a]; }
        int m = static_cast<unsigned char>(o[n_prga]);
        m = m + e + r[e] - a - r[a];
        m = m ^ (r[e] + r[a]);
        m = m ^ r[(r[e] + r[a]) & (rlen - 1)];
        m = m & 255;
        t.push_back(static_cast<char>(m));
        e = (e + 1) & (rlen - 1);
        n_prga += 1;
    }
    return base64_encode(t);
}

// ============================================================================
// zlib compress
// ============================================================================
std::vector<uint8_t> zlib_compress(const std::string& data) {
    z_stream zs{};
    if (deflateInit(&zs, Z_DEFAULT_COMPRESSION) != Z_OK)
        throw std::runtime_error("deflateInit failed");
    zs.next_in  = reinterpret_cast<Bytef*>(const_cast<char*>(data.data()));
    zs.avail_in = static_cast<uInt>(data.size());
    std::vector<uint8_t> out(data.size() + 1024);
    int ret = Z_OK;
    do {
        if (zs.total_out >= out.size()) out.resize(out.size() * 2);
        zs.next_out  = reinterpret_cast<Bytef*>(out.data() + zs.total_out);
        zs.avail_out = static_cast<uInt>(out.size() - zs.total_out);
        ret = deflate(&zs, Z_FINISH);
    } while (ret == Z_OK);
    out.resize(zs.total_out);
    deflateEnd(&zs);
    if (ret != Z_STREAM_END) throw std::runtime_error("deflate failed");
    return out;
}

// ============================================================================
// PART 8: VerifyCaptchaV3 — returns final payload or ""
// ============================================================================
std::string verify_captcha(const std::string& access_key,
                           const std::string& secret_key,
                           const std::string& scene_id,
                           const std::string& certify_id,
                           const std::string& data_value,
                           const std::string& device_token) {
    json cvp = {
        {"sceneId",     scene_id},
        {"certifyId",   certify_id},
        {"deviceToken", device_token},
        {"data",        data_value}
    };
    std::map<std::string, std::string> params = {
        {"AccessKeyId",      access_key},
        {"Action",           "VerifyCaptchaV3"},
        {"Format",           "JSON"},
        {"SignatureMethod",  "HMAC-SHA1"},
        {"SignatureVersion", "1.0"},
        {"Timestamp",        get_timestamp_utc()},
        {"Version",          "2023-03-05"},
        {"SceneId",          scene_id},
        {"CertifyId",        certify_id},
        {"CaptchaVerifyParam", cvp.dump()},
        {"SignatureNonce",   generate_uuid()},
    };
    std::string signature = generate_signature(params, secret_key);
    params["Signature"] = signature;

    std::string body;
    bool first = true;
    for (const auto& kv : params) {
        if (!first) body += "&";
        first = false;
        body += url_encode(kv.first) + "=" + url_encode(kv.second);
    }
    std::string response = http_post(
        "https://no8xfe-verify.captcha-open-southeast.aliyuncs.com/", body,
        {{"Referer", ""}});

    auto resp = json::parse(response);
    if (resp.value("Success", false)) {
        auto result = resp.value("Result", json::object());
        if (result.value("VerifyResult", false)) {
            std::string st = result.value("securityToken", "");
            std::string ci = result.value("certifyId", "");
            if (!st.empty() && !ci.empty()) {
                json fp = {
                    {"certifyId",    ci},
                    {"sceneId",      scene_id},
                    {"isSign",       true},
                    {"securityToken", st}
                };
                return base64_encode(fp.dump());
            }
            log_error("VerifyCaptchaV3 succeeded but securityToken/certifyId empty for deviceToken="
                      + device_token);
        } else {
            log_error("deviceToken failed verification (VerifyResult=false): " + device_token);
        }
    } else {
        log_error("VerifyCaptchaV3 request unsuccessful for deviceToken=" + device_token
                   + " response=" + response);
    }
    return "";
}

// ============================================================================
// Compute final payload on demand — tries tokens until success or exhausted
// ============================================================================
std::string compute_final_payload() {
    for (int attempt = 0; attempt < MAX_TOKEN_RETRIES; ++attempt) {
        std::string device_token;
        if (!get_next_token(device_token)) {
            log_error("No device tokens remaining in database (attempt "
                      + std::to_string(attempt + 1) + "/" + std::to_string(MAX_TOKEN_RETRIES) + ")");
            return "";
        }
        log_error("Attempt " + std::to_string(attempt + 1) + "/" + std::to_string(MAX_TOKEN_RETRIES)
                  + " using deviceToken=" + device_token);
        try {
            std::string certify_id = init_captcha(ACCESS_KEY, SECRET_KEY, SCENE_ID);
            std::string arg_value  = generate_arg(certify_id);

            int64_t ct = current_time_millis();
            json track = {
                {"TrackList", {
                    {"mc", ""}, {"tc", ""}, {"mu", ""}, {"te", ""},
                    {"mp", ""}, {"tmv", ""}, {"ks", ""}, {"fi", ""},
                    {"startTime", ct}
                }},
                {"TrackStartTime", ct},
                {"VerifyTime", ct + 300},
                {"arg", arg_value}
            };
            std::string json_str  = track.dump();
            std::string h         = ali_hash(json_str, "0000");
            std::string combined  = h + json_str;
            auto compressed       = zlib_compress(combined);
            std::string fb64      = base64_encode(compressed);
            std::string final_val = encrypt(
                std::vector<uint8_t>(fb64.begin(), fb64.end()));

            // Always remove token after use — prevents conflicts
            remove_token(device_token);

            std::string payload = verify_captcha(ACCESS_KEY, SECRET_KEY, SCENE_ID,
                                                  certify_id, final_val, device_token);
            if (!payload.empty()) return payload;
            // Token didn't work — retry with next
            log_error("deviceToken=" + device_token + " produced empty payload, retrying");
        } catch (const std::exception& e) {
            log_error("Attempt " + std::to_string(attempt + 1) + " failed for deviceToken="
                      + device_token + ": " + e.what());
            remove_token(device_token);
        }
    }
    log_error("All " + std::to_string(MAX_TOKEN_RETRIES) + " token retries exhausted");
    return "";
}

// ============================================================================
// Signal handler
// ============================================================================
static void signal_handler(int) { g_running = false; }

#ifdef _WIN32
// ============================================================================
// Named pipe server (Windows) — computes payload only when a client connects
// ============================================================================
void run_server() {
    std::signal(SIGINT,  signal_handler);
    std::signal(SIGTERM, signal_handler);

    std::string pipe_path = "\\\\.\\pipe\\" + PIPE_NAME;

    while (g_running) {
        HANDLE pipe = CreateNamedPipeA(
            pipe_path.c_str(),
            PIPE_ACCESS_OUTBOUND,
            PIPE_TYPE_BYTE | PIPE_WAIT,
            PIPE_UNLIMITED_INSTANCES,
            4096, 4096,
            0,
            nullptr);

        if (pipe == INVALID_HANDLE_VALUE) {
            log_error("CreateNamedPipe failed on '" + pipe_path + "', error=" +
                       std::to_string(GetLastError()));
            Sleep(1000);
            continue;
        }

        // ConnectNamedPipe blocks; we poll g_running by using overlapped-free
        // blocking connect and relying on process signals to break out.
        BOOL connected = ConnectNamedPipe(pipe, nullptr) ?
            TRUE : (GetLastError() == ERROR_PIPE_CONNECTED);

        if (!g_running) { CloseHandle(pipe); break; }

        if (!connected) {
            log_error("ConnectNamedPipe failed, error=" + std::to_string(GetLastError()));
            CloseHandle(pipe);
            continue;
        }

        // Compute payload only when asked
        std::string payload = compute_final_payload();
        std::string response = payload.empty() ? "ERROR" : payload;
        response += "\n";

        DWORD written = 0;
        if (!WriteFile(pipe, response.c_str(), (DWORD)response.size(), &written, nullptr)) {
            log_error("WriteFile to named pipe failed, error=" + std::to_string(GetLastError()));
        }

        FlushFileBuffers(pipe);
        DisconnectNamedPipe(pipe);
        CloseHandle(pipe);
    }
}
#else
// ============================================================================
// Named pipe server (POSIX FIFO pair) — computes payload only when a client
// writes to the request pipe, then responds on the response pipe.
// Two separate FIFOs are used (request/response) because a single FIFO with
// multiple potential readers/writers is subject to read races between the
// server's "is anyone connecting" probe and the actual client.
// ============================================================================
void run_server() {
    signal(SIGPIPE, SIG_IGN);
    std::signal(SIGINT,  signal_handler);
    std::signal(SIGTERM, signal_handler);

    std::string req_path  = "/tmp/" + PIPE_NAME + ".req";
    std::string resp_path = "/tmp/" + PIPE_NAME + ".resp";

    unlink(req_path.c_str());
    unlink(resp_path.c_str());
    if (mkfifo(req_path.c_str(), 0666) != 0 && errno != EEXIST) {
        log_error("mkfifo failed on '" + req_path + "': " + std::strerror(errno));
        return;
    }
    if (mkfifo(resp_path.c_str(), 0666) != 0 && errno != EEXIST) {
        log_error("mkfifo failed on '" + resp_path + "': " + std::strerror(errno));
        unlink(req_path.c_str());
        return;
    }

    while (g_running) {
        // Open request pipe non-blocking so we can poll g_running while idle
        int rfd = open(req_path.c_str(), O_RDONLY | O_NONBLOCK);
        if (rfd < 0) {
            if (errno != EINTR) {
                log_error("open() on request FIFO '" + req_path + "' failed: " + std::strerror(errno));
            }
            usleep(200000);
            continue;
        }

        // Switch to blocking mode to wait for an actual client to write
        int flags = fcntl(rfd, F_GETFL, 0);
        fcntl(rfd, F_SETFL, flags & ~O_NONBLOCK);

        char buf[256];
        ssize_t n = read(rfd, buf, sizeof(buf));
        close(rfd);

        if (n <= 0) continue; // no real client request, loop and re-check g_running

        // Compute payload only when asked
        std::string payload = compute_final_payload();
        std::string response = payload.empty() ? "ERROR" : payload;
        response += "\n";

        int wfd = open(resp_path.c_str(), O_WRONLY);
        if (wfd < 0) {
            log_error("Failed to open response FIFO '" + resp_path + "' for write: " + std::strerror(errno));
            continue;
        }
        ssize_t written = write(wfd, response.c_str(), response.size());
        if (written < 0 || static_cast<size_t>(written) != response.size()) {
            log_error("write() to response FIFO failed or incomplete: " + std::string(std::strerror(errno)));
        }
        close(wfd);
    }

    unlink(req_path.c_str());
    unlink(resp_path.c_str());
}
#endif

// ============================================================================
// Argument parsing
// ============================================================================
static void print_usage(const char* prog) {
    std::cerr << "Usage: " << prog << " [--db-path /path/to/tokens.sqlite] [--verbose]\n";
}

static bool parse_args(int argc, char** argv) {
    for (int i = 1; i < argc; ++i) {
        std::string arg = argv[i];
        if (arg == "--db-path") {
            if (i + 1 >= argc) {
                std::cerr << "Error: --db-path requires a value\n";
                return false;
            }
            DB_PATH = argv[++i];
        } else if (arg.rfind("--db-path=", 0) == 0) {
            DB_PATH = arg.substr(std::string("--db-path=").size());
        } else if (arg == "--verbose") {
            g_verbose = true;
        } else if (arg == "-h" || arg == "--help") {
            print_usage(argv[0]);
            return false;
        } else {
            std::cerr << "Unknown argument: " << arg << "\n";
            print_usage(argv[0]);
            return false;
        }
    }
    return true;
}

// ============================================================================
// Main — silent background server (errors logged only with --verbose)
// ============================================================================
int main(int argc, char** argv) {
    if (!parse_args(argc, argv)) {
        return 1;
    }
    log_error("Starting with db-path='" + DB_PATH + "' verbose=true");
    curl_global_init(CURL_GLOBAL_ALL);
    run_server();
    curl_global_cleanup();
    return 0;
}
