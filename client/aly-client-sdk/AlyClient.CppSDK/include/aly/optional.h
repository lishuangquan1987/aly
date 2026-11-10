#pragma once

namespace aly {

/// C++11 可选值；XP 兼容，不依赖 std::optional。
template <typename T>
class Optional {
public:
    Optional() : present_(false), value_() {}
    Optional(const T& value) : present_(true), value_(value) {}

    bool has_value() const { return present_; }
    explicit operator bool() const { return present_; }

    const T& get() const { return value_; }
    T& get() { return value_; }

    T value_or(const T& fallback) const { return present_ ? value_ : fallback; }

private:
    bool present_;
    T value_;
};

} // namespace aly