#include <openssl/hmac.h>
#include <openssl/evp.h>
#include <openssl/bio.h>
#include <openssl/buffer.h>

#include <curl/curl.h>
#include <nlohmann/json.hpp>
#include <zlib.h>
                                                                                                                                                                          #include <algorithm>
#include <array>
#include <cstdint>
#include <chrono>
#include <cstring>
#include <ctime>
#include <iomanip>
#include <iostream>
#include <map>
#include <random>
#include <set>
#include <sstream>
#include <string>
#include <vector>

using json = nlohmann::json;
                                                                                                                                                                          // ============================================================================
// Utility: URL encoding (component-wise, like Python's quote with safe='')
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
    std::string s(data.begin(), data.end());
    return base64_encode(s);
}

// ============================================================================
// Utility: Hex encode
// ============================================================================
std::string hex_encode(const uint8_t* data, size_t len) {
    std::ostringstream oss;
    oss << std::hex << std::setfill('0');
    for (size_t i = 0; i < len; ++i) {
        oss << std::setw(2) << static_cast<int>(data[i]);
    }
    return oss.str();
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
// UUID v4 generation
// ============================================================================
std::string generate_uuid() {
    static std::random_device rd;
    static std::mt19937_64 gen(rd());
    std::uniform_int_distribution<uint64_t> dis(0, UINT64_MAX);
    uint64_t a = dis(gen);
    uint64_t b = dis(gen);
    // Version 4, Variant 1
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
// UTC timestamp in ISO 8601 format
// ============================================================================
std::string get_timestamp_utc() {
    auto now = std::chrono::system_clock::now();
    std::time_t now_t = std::chrono::system_clock::to_time_t(now);
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

// Current time in milliseconds since epoch
int64_t current_time_millis() {
    auto now = std::chrono::system_clock::now();
    auto ms = std::chrono::duration_cast<std::chrono::milliseconds>(
                  now.time_since_epoch());
    return static_cast<int64_t>(ms.count());
}

// ============================================================================
// Generate Aliyun signature
// ============================================================================
std::string generate_signature(const std::map<std::string, std::string>& params,
                               const std::string& secret_key) {
    // 1. Build canonicalized query string
    std::vector<std::pair<std::string, std::string>> sorted_params(
        params.begin(), params.end());
    std::sort(sorted_params.begin(), sorted_params.end(),
              [](const auto& a, const auto& b) { return a.first < b.first; });

    std::string canonical;
    for (size_t i = 0; i < sorted_params.size(); ++i) {
        if (i > 0) canonical += "&";
        canonical += url_encode(sorted_params[i].first);
        canonical += "=";
        canonical += url_encode(sorted_params[i].second);
    }

    // 2. Build string to sign: POST&%2F&<url-encoded canonical>
    std::string string_to_sign = "POST&" + url_encode("/") + "&" +
                                 url_encode(canonical);

    // 3. HMAC-SHA1 with key = secret_key + "&"
    std::string signing_key = secret_key + "&";
    std::string digest = hmac_sha1(signing_key, string_to_sign);

    return base64_encode(digest);
}

// ============================================================================
// libcurl write callback
// ============================================================================
static size_t write_callback(void* contents, size_t size, size_t nmemb,
                             void* userp) {
    size_t total_size = size * nmemb;
    std::string* str = static_cast<std::string*>(userp);
    str->append(static_cast<char*>(contents), total_size);
    return total_size;
}

// ============================================================================
// HTTP POST helper
// ============================================================================
std::string http_post(const std::string& url,
                      const std::string& body,
                      const std::vector<std::pair<std::string, std::string>>&
                          extra_headers = {}) {
    CURL* curl = curl_easy_init();
    if (!curl) {
        throw std::runtime_error("Failed to init curl");
    }
    std::string response_data;

    struct curl_slist* headers = nullptr;
    headers = curl_slist_append(headers,
        "Content-Type: application/x-www-form-urlencoded; charset=UTF-8");
    for (const auto& h : extra_headers) {
        headers = curl_slist_append(headers, (h.first + ": " + h.second).c_str());
    }

    curl_easy_setopt(curl, CURLOPT_URL, url.c_str());
    curl_easy_setopt(curl, CURLOPT_POST, 1L);
    curl_easy_setopt(curl, CURLOPT_POSTFIELDS, body.c_str());
    curl_easy_setopt(curl, CURLOPT_POSTFIELDSIZE, body.size());
    curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_callback);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &response_data);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, 30L);
    curl_easy_setopt(curl, CURLOPT_SSL_VERIFYPEER, 1L);

    CURLcode res = curl_easy_perform(curl);
    long http_code = 0;
    curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &http_code);

    curl_slist_free_all(headers);
    curl_easy_cleanup(curl);

    if (res != CURLE_OK) {
        throw std::runtime_error(std::string("curl error: ") + curl_easy_strerror(res));
    }
    return response_data;
}

// ============================================================================
// Configuration
// ============================================================================
static const std::string ACCESS_KEY = "LTAI5tSEBwYMwVKAQGpxmvTd";
static const std::string SECRET_KEY = "YSKfst7GaVkXwZYvVihJsKF9r89koz";
static const std::string SCENE_ID = "didk33e0";

// ============================================================================
// PART 1: InitCaptchaV3
// ============================================================================
std::string init_captcha(const std::string& access_key,
                         const std::string& secret_key,
                         const std::string& scene_id) {
    std::map<std::string, std::string> params = {
        {"AccessKeyId", access_key},
        {"Action", "InitCaptchaV3"},
        {"Format", "JSON"},
        {"Language", "en"},
        {"Mode", "popup"},
        {"SceneId", scene_id},
        {"SignatureMethod", "HMAC-SHA1"},
        {"SignatureNonce", generate_uuid()},
        {"SignatureVersion", "1.0"},
        {"Timestamp", get_timestamp_utc()},
        {"UpLang", "true"},
        {"Version", "2023-03-05"},
    };

    std::string signature = generate_signature(params, secret_key);
    params["Signature"] = signature;

    // Build form-encoded body
    std::string body;
    bool first = true;
    for (const auto& kv : params) {
        if (!first) body += "&";
        first = false;
        body += url_encode(kv.first) + "=" + url_encode(kv.second);
    }

    std::string response = http_post(
        "https://no8xfe.captcha-open-southeast.aliyuncs.com/", body);

    auto resp_json = json::parse(response);
    std::cout << "Response:\n" << resp_json.dump(2) << "\n\n";

    std::string certify_id = resp_json["CertifyId"].get<std::string>();
    std::cout << "CertifyId: " << certify_id << "\n";
    return certify_id;
}

// ============================================================================
// PART 2: Generate arg field
// ============================================================================
std::string generate_arg(const std::string& certify_id,
                         const std::string& constant = "4xrihv8zb8tf1mfj") {
    // URL-decode the percent-encoded version of certify_id
    // Equivalent to Python: urllib.parse.quote(certify_id) then decode %xx
    std::string encoded = url_encode(certify_id);
    std::string o;
    for (size_t i = 0; i < encoded.size();) {
        if (encoded[i] == '%' && i + 2 < encoded.size()) {
            int hi = std::stoi(encoded.substr(i + 1, 2), nullptr, 16);
            o.push_back(static_cast<char>(hi));
            i += 3;
        } else {
            o.push_back(encoded[i]);
            i += 1;
        }
    }

    std::string n = constant;
    std::vector<int> r = {
        32,50,10,51,6,44,37,16,46,11,62,19,43,25,23,30,
        60,33,53,34,7,26,12,48,5,2,20,4,61,13,47,49,
        18,29,27,22,1,17,39,56,41,38,55,31,15,58,52,40,
        8,57,45,35,59,36,42,54,63,3,24,28,14,9,0,21
    };
    int rlen = static_cast<int>(r.size());  // 64

    int i = 0, j = 0;
    while (i < rlen) {
        j = (((i + j + r[i] + r[j]) >> 1) + n[i % n.size()]) & (rlen - 1);
        if (i != j) {
            r[i] ^= r[j];
            r[j] ^= r[i];
            r[i] ^= r[j];
        }
        i += 1;
    }

    std::string t;
    int e = 0, a = 0;
    for (size_t idx = 0; idx < o.size(); ++idx) {
        a = ((e ^ a) + (r[e] ^ r[a])) & (rlen - 1);
        if (e != a) {
            r[e] ^= r[a];
            r[a] ^= r[e];
            r[e] ^= r[a];
        }
        int m = static_cast<unsigned char>(o[idx]);
        m = m + e + r[e] - a - r[a];
        m = m ^ (r[e] + r[a]);
        m = m ^ r[(r[e] + r[a]) & (rlen - 1)];
        m = m & 255;
        t.push_back(static_cast<char>(m));
        e = (e + 1) & (rlen - 1);
    }

    // base64 encode treating bytes as latin-1 (i.e. byte-wise)
    return base64_encode(t);
}

// ============================================================================
// PART 4: ali_hash
// ============================================================================
std::string ali_hash(const std::string& input_str, const std::string& salt_str) {
    // Treat input as UTF-8 -> decode to latin-1 bytes (each char -> byte).
    // For ASCII-compatible strings this is a no-op; for non-ASCII bytes we
    // take low byte of each char (Python's encode('utf-8').decode('latin-1')).
    std::string o = input_str;
    std::string r = salt_str;
    int a_len = static_cast<int>(o.size());
    int m = static_cast<int>(r.size());

    std::vector<int> e;
    for (int i = 0; i < 16; ++i) {
        e.push_back((i << 4) + (i % 16));
    }
    int f = static_cast<int>(e.size());  // 16

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
        if (pos != 0) {
            e[pos] ^= e[pos - 1];
        } else {
            e[0] ^= e[f - 1];
        }
    }

    std::ostringstream result;
    for (int b : e) {
        result << std::hex << std::setw(2) << std::setfill('0') << (b & 0xFF);
    }
    return result.str();
}

// ============================================================================
// PART 7: encrypt
// ============================================================================
std::string encrypt(const std::vector<uint8_t>& plaintext_bytes) {
    // Convert bytes to a "latin-1" string: each byte -> char
    std::string o(plaintext_bytes.begin(), plaintext_bytes.end());

    std::string n = "3e627e1b4c63f913";
    std::vector<int> r = {
        32,50,10,51,6,44,37,16,46,11,62,19,43,25,23,30,
        60,33,53,34,7,26,12,48,5,2,20,4,61,13,47,49,
        18,29,27,22,1,17,39,56,41,38,55,31,15,58,52,40,
        8,57,45,35,59,36,42,54,63,3,24,28,14,9,0,21
    };
    int rlen = static_cast<int>(r.size());  // 64

    int o_ksa = 0, t_ksa = 0;
    while (o_ksa < rlen) {
        t_ksa = (((o_ksa + t_ksa + r[o_ksa] + r[t_ksa]) >> 1)
                 + n[o_ksa % n.size()]) & (rlen - 1);
        if (o_ksa != t_ksa) {
            r[o_ksa] ^= r[t_ksa];
            r[t_ksa] ^= r[o_ksa];
            r[o_ksa] ^= r[t_ksa];
        }
        o_ksa += 1;
    }

    std::string t;
    int n_prga = 0, e = 0, a = 0;
    while (n_prga < static_cast<int>(o.size())) {
        a = ((e ^ a) + (r[e] ^ r[a])) & (rlen - 1);
        if (e != a) {
            r[e] ^= r[a];
            r[a] ^= r[e];
            r[e] ^= r[a];
        }
        int m = static_cast<unsigned char>(o[n_prga]);
        m = m + e + r[e];
        m = m - a - r[a];
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
// zlib compress (default level)
// ============================================================================
std::vector<uint8_t> zlib_compress(const std::string& data) {
    z_stream zs{};
    if (deflateInit(&zs, Z_DEFAULT_COMPRESSION) != Z_OK) {
        throw std::runtime_error("deflateInit failed");
    }
    zs.next_in = reinterpret_cast<Bytef*>(const_cast<char*>(data.data()));
    zs.avail_in = static_cast<uInt>(data.size());

    std::vector<uint8_t> out;
    out.resize(data.size() + 1024);  // Initial buffer
    int ret = Z_OK;
    do {
        if (zs.total_out >= out.size()) {
            out.resize(out.size() * 2);
        }
        zs.next_out = reinterpret_cast<Bytef*>(out.data() + zs.total_out);
        zs.avail_out = static_cast<uInt>(out.size() - zs.total_out);
        ret = deflate(&zs, Z_FINISH);
    } while (ret == Z_OK);

    out.resize(zs.total_out);
    deflateEnd(&zs);

    if (ret != Z_STREAM_END) {
        throw std::runtime_error("deflate failed");
    }
    return out;
}

// ============================================================================
// PART 8: VerifyCaptchaV3
// ============================================================================
void verify_captcha(const std::string& access_key,
                    const std::string& secret_key,
                    const std::string& scene_id,
                    const std::string& certify_id,
                    const std::string& data_value,
                    const std::string& device_token) {
    // Build CaptchaVerifyParam JSON
    json captcha_verify_param = {
        {"sceneId", scene_id},
        {"certifyId", certify_id},
        {"deviceToken", device_token},
        {"data", data_value}
    };
    std::string captcha_verify_param_str =
        captcha_verify_param.dump();  // compact JSON

    std::map<std::string, std::string> params = {
        {"AccessKeyId", access_key},
        {"Action", "VerifyCaptchaV3"},
        {"Format", "JSON"},
        {"SignatureMethod", "HMAC-SHA1"},
        {"SignatureVersion", "1.0"},
        {"Timestamp", get_timestamp_utc()},
        {"Version", "2023-03-05"},
        {"SceneId", scene_id},
        {"CertifyId", certify_id},
        {"CaptchaVerifyParam", captcha_verify_param_str},
        {"SignatureNonce", generate_uuid()},
    };

    std::string signature = generate_signature(params, secret_key);
    params["Signature"] = signature;

    // Build body
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

    std::cout << "\n=== Response ===\n";
    auto resp_json = json::parse(response);
    std::cout << resp_json.dump(2) << "\n";

    if (resp_json.value("Success", false)) {
        auto result = resp_json.value("Result", json::object());
        if (result.value("VerifyResult", false)) {
            std::cout << "\n✅ CAPTCHA Verification SUCCESSFUL!\n";
            std::cout << "VerifyCode: "
                      << result.value("VerifyCode", "") << "\n";

            std::string security_token =
                result.value("securityToken", "");
            std::string resp_certify_id =
                result.value("certifyId", "");

            if (!security_token.empty() && !resp_certify_id.empty()) {
                json final_payload = {
                    {"certifyId", resp_certify_id},
                    {"sceneId", scene_id},
                    {"isSign", true},
                    {"securityToken", security_token}
                };
                std::string final_payload_json = final_payload.dump();
                std::string final_payload_b64 =
                    base64_encode(final_payload_json);

                std::cout << "\n" << std::string(60, '=') << "\n";
                std::cout << "FINAL BASE64 ENCODED PAYLOAD:\n";
                std::cout << std::string(60, '=') << "\n";
                std::cout << final_payload_b64 << "\n";
            } else {
                std::cout << "\n⚠️ securityToken or certifyId missing\n";
            }
        } else {
            std::cout << "\n❌ CAPTCHA Verification FAILED\n";
            std::cout << "Message: " << resp_json.value("Message", "") << "\n";
        }
    } else {
        std::cout << "\n❌ CAPTCHA Verification FAILED\n";
        std::cout << "Message: " << resp_json.value("Message", "") << "\n";
    }
}

// ============================================================================
// Main
// ============================================================================
int main() {
    curl_global_init(CURL_GLOBAL_ALL);

    try {
        std::cout << "=== PART 1: InitCaptchaV3 ===\n";
        std::string certify_id = init_captcha(ACCESS_KEY, SECRET_KEY, SCENE_ID);

        std::cout << "\n=== PART 2: Generate arg ===\n";
        std::string arg_value = generate_arg(certify_id);
        std::cout << "arg: " << arg_value << "\n";

        std::cout << "\n=== PART 3: Build Track JSON ===\n";
        int64_t ct = current_time_millis();
        json track_json = {
            {"TrackList", {
                {"mc", ""}, {"tc", ""}, {"mu", ""}, {"te", ""},
                {"mp", ""}, {"tmv", ""}, {"ks", ""}, {"fi", ""},
                {"startTime", ct}
            }},
            {"TrackStartTime", ct},
            {"VerifyTime", ct + 300},
            {"arg", arg_value}
        };
        std::string json_str = track_json.dump();
        std::cout << "JSON: " << json_str << "\n";

        std::cout << "\n=== PART 4: Calculate hash ===\n";
        std::string hash_result = ali_hash(json_str, "0000");
        std::cout << "Hash: " << hash_result << "\n";

        std::cout << "\n=== PART 5: Combine hash + json ===\n";
        std::string combined = hash_result + json_str;
        std::cout << "Combined (first 100): "
                  << combined.substr(0, 100) << "...\n";

        std::cout << "\n=== PART 6: zlib compress ===\n";
        std::vector<uint8_t> compressed = zlib_compress(combined);
        std::cout << "Compressed size: " << compressed.size() << " bytes\n";

        std::cout << "\n=== PART 7: Final encryption ===\n";
        std::string final_data_b64 = base64_encode(compressed);
        std::string final_value = encrypt(
            std::vector<uint8_t>(final_data_b64.begin(), final_data_b64.end()));

        std::cout << "\n" << std::string(60, '=') << "\n";
        std::cout << "FINAL VALUE (captchaVerifyParam data):\n";
        std::cout << std::string(60, '=') << "\n";
        std::cout << final_value << "\n";
        std::cout << std::string(60, '=') << "\n";

        std::cout << "\n=== PART 8: VerifyCaptchaV3 ===\n";
        std::cout << "Please provide the DeviceToken for VerifyCaptchaV3:\n";
        std::cout << "Enter DeviceToken: ";
        std::string device_token;
        std::getline(std::cin, device_token);

        verify_captcha(ACCESS_KEY, SECRET_KEY, SCENE_ID,
                       certify_id, final_value, device_token);

    } catch (const std::exception& e) {
        std::cerr << "Error: " << e.what() << "\n";
        curl_global_cleanup();
        return 1;
    }

    curl_global_cleanup();
    return 0;
}
