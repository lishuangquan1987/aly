#include "aly/json.h"

#include <cstdlib>
#include <stdexcept>

namespace aly {

class Parser {
public:
    explicit Parser(const std::string& text) : text_(text), pos_(0) {}

    JsonValue parse() {
        JsonValue value = parseValue();
        skipWhitespace();
        if (pos_ != text_.size()) {
            fail("trailing JSON content");
        }
        return value;
    }

private:
    JsonValue parseValue() {
        skipWhitespace();
        if (pos_ >= text_.size()) {
            fail("unexpected end of JSON");
        }
        switch (text_[pos_]) {
        case '{':
            return parseObject();
        case '[':
            return parseArray();
        case '"': {
            JsonValue value;
            value.type_ = JsonValue::String;
            value.string_ = parseString();
            return value;
        }
        case 't':
            expect("true");
            {
                JsonValue value;
                value.type_ = JsonValue::Bool;
                value.bool_ = true;
                return value;
            }
        case 'f':
            expect("false");
            {
                JsonValue value;
                value.type_ = JsonValue::Bool;
                value.bool_ = false;
                return value;
            }
        case 'n':
            expect("null");
            return JsonValue();
        default:
            if (text_[pos_] == '-' || (text_[pos_] >= '0' && text_[pos_] <= '9')) {
                return parseNumber();
            }
            fail("unexpected character");
        }
        return JsonValue();
    }

    JsonValue parseObject() {
        JsonValue value;
        value.type_ = JsonValue::Object;
        ++pos_; // {
        skipWhitespace();
        if (pos_ < text_.size() && text_[pos_] == '}') {
            ++pos_;
            return value;
        }
        for (;;) {
            skipWhitespace();
            if (pos_ >= text_.size() || text_[pos_] != '"') {
                fail("expected object key");
            }
            std::string key = parseString();
            skipWhitespace();
            if (pos_ >= text_.size() || text_[pos_] != ':') {
                fail("expected ':' after key");
            }
            ++pos_;
            value.object_[key] = parseValue();
            skipWhitespace();
            if (pos_ >= text_.size()) {
                fail("unterminated object");
            }
            char next = text_[pos_++];
            if (next == '}') {
                return value;
            }
            if (next != ',') {
                fail("expected ',' or '}'");
            }
        }
    }

    JsonValue parseArray() {
        JsonValue value;
        value.type_ = JsonValue::Array;
        ++pos_; // [
        skipWhitespace();
        if (pos_ < text_.size() && text_[pos_] == ']') {
            ++pos_;
            return value;
        }
        for (;;) {
            value.array_.push_back(parseValue());
            skipWhitespace();
            if (pos_ >= text_.size()) {
                fail("unterminated array");
            }
            char next = text_[pos_++];
            if (next == ']') {
                return value;
            }
            if (next != ',') {
                fail("expected ',' or ']'");
            }
        }
    }

    std::string parseString() {
        ++pos_; // opening quote
        std::string out;
        while (pos_ < text_.size()) {
            unsigned char c = static_cast<unsigned char>(text_[pos_++]);
            if (c == '"') {
                return out;
            }
            if (c == '\\') {
                if (pos_ >= text_.size()) {
                    fail("unterminated escape");
                }
                char esc = text_[pos_++];
                switch (esc) {
                case '"':
                    out.push_back('"');
                    break;
                case '\\':
                    out.push_back('\\');
                    break;
                case '/':
                    out.push_back('/');
                    break;
                case 'b':
                    out.push_back('\b');
                    break;
                case 'f':
                    out.push_back('\f');
                    break;
                case 'n':
                    out.push_back('\n');
                    break;
                case 'r':
                    out.push_back('\r');
                    break;
                case 't':
                    out.push_back('\t');
                    break;
                case 'u':
                    appendUnicode(out, parseHex4());
                    break;
                default:
                    fail("bad escape");
                }
            } else {
                out.push_back(static_cast<char>(c));
            }
        }
        fail("unterminated string");
        return out;
    }

    unsigned int parseHex4() {
        if (pos_ + 4 > text_.size()) {
            fail("bad unicode escape");
        }
        unsigned int value = 0;
        for (int i = 0; i < 4; ++i) {
            char c = text_[pos_++];
            value <<= 4;
            if (c >= '0' && c <= '9') {
                value |= static_cast<unsigned int>(c - '0');
            } else if (c >= 'a' && c <= 'f') {
                value |= static_cast<unsigned int>(c - 'a' + 10);
            } else if (c >= 'A' && c <= 'F') {
                value |= static_cast<unsigned int>(c - 'A' + 10);
            } else {
                fail("bad hex digit");
            }
        }
        return value;
    }

    void appendUnicode(std::string& out, unsigned int code) {
        if (code >= 0xD800 && code <= 0xDBFF && pos_ + 6 <= text_.size()
            && text_[pos_] == '\\' && text_[pos_ + 1] == 'u') {
            pos_ += 2;
            unsigned int low = parseHex4();
            if (low >= 0xDC00 && low <= 0xDFFF) {
                code = 0x10000 + ((code - 0xD800) << 10) + (low - 0xDC00);
            } else {
                appendUtf8(out, code);
                code = low;
            }
        }
        appendUtf8(out, code);
    }

    static void appendUtf8(std::string& out, unsigned int code) {
        if (code <= 0x7F) {
            out.push_back(static_cast<char>(code));
        } else if (code <= 0x7FF) {
            out.push_back(static_cast<char>(0xC0 | (code >> 6)));
            out.push_back(static_cast<char>(0x80 | (code & 0x3F)));
        } else if (code <= 0xFFFF) {
            out.push_back(static_cast<char>(0xE0 | (code >> 12)));
            out.push_back(static_cast<char>(0x80 | ((code >> 6) & 0x3F)));
            out.push_back(static_cast<char>(0x80 | (code & 0x3F)));
        } else {
            out.push_back(static_cast<char>(0xF0 | (code >> 18)));
            out.push_back(static_cast<char>(0x80 | ((code >> 12) & 0x3F)));
            out.push_back(static_cast<char>(0x80 | ((code >> 6) & 0x3F)));
            out.push_back(static_cast<char>(0x80 | (code & 0x3F)));
        }
    }

    JsonValue parseNumber() {
        size_t start = pos_;
        if (text_[pos_] == '-') {
            ++pos_;
        }
        while (pos_ < text_.size()) {
            char c = text_[pos_];
            if ((c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-') {
                ++pos_;
            } else {
                break;
            }
        }
        std::string raw = text_.substr(start, pos_ - start);
        JsonValue value;
        value.type_ = JsonValue::Number;
        if (raw.find('.') == std::string::npos && raw.find('e') == std::string::npos
            && raw.find('E') == std::string::npos) {
            value.number_ = static_cast<double>(std::strtoll(raw.c_str(), NULL, 10));
        } else {
            value.number_ = std::strtod(raw.c_str(), NULL);
        }
        return value;
    }

    void expect(const char* literal) {
        size_t length = std::char_traits<char>::length(literal);
        if (text_.compare(pos_, length, literal) != 0) {
            fail("expected literal");
        }
        pos_ += length;
    }

    void skipWhitespace() {
        while (pos_ < text_.size()) {
            char c = text_[pos_];
            if (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
                ++pos_;
            } else {
                break;
            }
        }
    }

    void fail(const char* message) {
        throw std::runtime_error(std::string(message) + " at " + std::to_string(pos_));
    }

    const std::string& text_;
    size_t pos_;
};

const JsonValue& JsonValue::operator[](const std::string& key) const {
    static const JsonValue nullValue;
    if (type_ != Object) {
        return nullValue;
    }
    std::map<std::string, JsonValue>::const_iterator it = object_.find(key);
    return it == object_.end() ? nullValue : it->second;
}

JsonValue parseJson(const std::string& text) {
    Parser parser(text);
    return parser.parse();
}

bool jsonBool(const JsonValue& obj, const char* key, bool fallback) {
    if (!obj.has(key) || obj[key].type() != JsonValue::Bool) {
        return fallback;
    }
    return obj[key].asBool();
}

Optional<bool> jsonOptBool(const JsonValue& obj, const char* key) {
    if (!obj.has(key) || obj[key].type() != JsonValue::Bool) {
        return Optional<bool>();
    }
    return Optional<bool>(obj[key].asBool());
}

std::string jsonString(const JsonValue& obj, const char* key) {
    if (!obj.has(key) || obj[key].type() != JsonValue::String) {
        return std::string();
    }
    return obj[key].asString();
}

Optional<std::string> jsonOptString(const JsonValue& obj, const char* key) {
    if (!obj.has(key) || obj[key].type() != JsonValue::String) {
        return Optional<std::string>();
    }
    return Optional<std::string>(obj[key].asString());
}

long long jsonInt(const JsonValue& obj, const char* key, long long fallback) {
    if (!obj.has(key) || obj[key].type() != JsonValue::Number) {
        return fallback;
    }
    return obj[key].asInt();
}

Optional<long long> jsonOptInt(const JsonValue& obj, const char* key) {
    if (!obj.has(key) || obj[key].type() != JsonValue::Number) {
        return Optional<long long>();
    }
    return Optional<long long>(obj[key].asInt());
}

double jsonDouble(const JsonValue& obj, const char* key, double fallback) {
    if (!obj.has(key) || obj[key].type() != JsonValue::Number) {
        return fallback;
    }
    return obj[key].asDouble();
}

Optional<double> jsonOptDouble(const JsonValue& obj, const char* key) {
    if (!obj.has(key) || obj[key].type() != JsonValue::Number) {
        return Optional<double>();
    }
    return Optional<double>(obj[key].asDouble());
}

std::vector<std::string> jsonStringList(const JsonValue& obj, const char* key) {
    std::vector<std::string> out;
    if (!obj.has(key) || !obj[key].isArray()) {
        return out;
    }
    const std::vector<JsonValue>& array = obj[key].asArray();
    for (size_t i = 0; i < array.size(); ++i) {
        if (array[i].type() == JsonValue::String) {
            out.push_back(array[i].asString());
        }
    }
    return out;
}

Optional<std::vector<std::string> > jsonOptStringList(const JsonValue& obj, const char* key) {
    if (!obj.has(key) || !obj[key].isArray()) {
        return Optional<std::vector<std::string> >();
    }
    return Optional<std::vector<std::string> >(jsonStringList(obj, key));
}

std::vector<bool> jsonBoolList(const JsonValue& obj, const char* key) {
    std::vector<bool> out;
    if (!obj.has(key) || !obj[key].isArray()) {
        return out;
    }
    const std::vector<JsonValue>& array = obj[key].asArray();
    for (size_t i = 0; i < array.size(); ++i) {
        if (array[i].type() == JsonValue::Bool) {
            out.push_back(array[i].asBool());
        }
    }
    return out;
}

std::vector<long long> jsonIntList(const JsonValue& obj, const char* key) {
    std::vector<long long> out;
    if (!obj.has(key) || !obj[key].isArray()) {
        return out;
    }
    const std::vector<JsonValue>& array = obj[key].asArray();
    for (size_t i = 0; i < array.size(); ++i) {
        if (array[i].type() == JsonValue::Number) {
            out.push_back(array[i].asInt());
        }
    }
    return out;
}

std::vector<double> jsonDoubleList(const JsonValue& obj, const char* key) {
    std::vector<double> out;
    if (!obj.has(key) || !obj[key].isArray()) {
        return out;
    }
    const std::vector<JsonValue>& array = obj[key].asArray();
    for (size_t i = 0; i < array.size(); ++i) {
        if (array[i].type() == JsonValue::Number) {
            out.push_back(array[i].asDouble());
        }
    }
    return out;
}

} // namespace aly