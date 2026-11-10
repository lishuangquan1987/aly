#pragma once

#include <map>
#include <string>
#include <vector>

#include "optional.h"

namespace aly {

class Parser;

/// 极简 JSON 值（对象/数组/字符串/数字/布尔/null），UTF-8 字符串。
class JsonValue {
public:
    enum Type { Null, Bool, Number, String, Array, Object };

    JsonValue() : type_(Null), bool_(false), number_(0.0) {}

    Type type() const { return type_; }
    bool isNull() const { return type_ == Null; }
    bool isObject() const { return type_ == Object; }
    bool isArray() const { return type_ == Array; }

    bool asBool() const { return bool_; }
    long long asInt() const { return static_cast<long long>(number_); }
    double asDouble() const { return number_; }
    const std::string& asString() const { return string_; }
    const std::vector<JsonValue>& asArray() const { return array_; }
    const std::map<std::string, JsonValue>& asObject() const { return object_; }

    bool has(const std::string& key) const {
        return type_ == Object && object_.find(key) != object_.end();
    }

    const JsonValue& operator[](const std::string& key) const;

private:
    friend class Parser;
    Type type_;
    bool bool_;
    double number_;
    std::string string_;
    std::vector<JsonValue> array_;
    std::map<std::string, JsonValue> object_;
};

/// 解析 UTF-8 JSON 文本；失败抛 std::runtime_error。
JsonValue parseJson(const std::string& text);

bool jsonBool(const JsonValue& obj, const char* key, bool fallback);
Optional<bool> jsonOptBool(const JsonValue& obj, const char* key);

std::string jsonString(const JsonValue& obj, const char* key);
Optional<std::string> jsonOptString(const JsonValue& obj, const char* key);

long long jsonInt(const JsonValue& obj, const char* key, long long fallback);
Optional<long long> jsonOptInt(const JsonValue& obj, const char* key);

double jsonDouble(const JsonValue& obj, const char* key, double fallback);
Optional<double> jsonOptDouble(const JsonValue& obj, const char* key);

std::vector<std::string> jsonStringList(const JsonValue& obj, const char* key);
Optional<std::vector<std::string> > jsonOptStringList(const JsonValue& obj, const char* key);

std::vector<bool> jsonBoolList(const JsonValue& obj, const char* key);
std::vector<long long> jsonIntList(const JsonValue& obj, const char* key);
std::vector<double> jsonDoubleList(const JsonValue& obj, const char* key);

template <typename T>
Optional<T> jsonObject(const JsonValue& obj, const char* key, void (*fromJson)(const JsonValue&, T&)) {
    if (!obj.has(key)) {
        return Optional<T>();
    }
    const JsonValue& value = obj[key];
    if (!value.isObject()) {
        return Optional<T>();
    }
    T out;
    fromJson(value, out);
    return Optional<T>(out);
}

template <typename T>
std::vector<T> jsonObjectList(
    const JsonValue& obj, const char* key, void (*fromJson)(const JsonValue&, T&)) {
    std::vector<T> out;
    if (!obj.has(key) || !obj[key].isArray()) {
        return out;
    }
    const std::vector<JsonValue>& array = obj[key].asArray();
    for (size_t i = 0; i < array.size(); ++i) {
        if (!array[i].isObject()) {
            continue;
        }
        T item;
        fromJson(array[i], item);
        out.push_back(item);
    }
    return out;
}

template <typename T>
Optional<std::vector<T> > jsonOptObjectList(
    const JsonValue& obj, const char* key, void (*fromJson)(const JsonValue&, T&)) {
    if (!obj.has(key) || !obj[key].isArray()) {
        return Optional<std::vector<T> >();
    }
    return Optional<std::vector<T> >(jsonObjectList<T>(obj, key, fromJson));
}

} // namespace aly