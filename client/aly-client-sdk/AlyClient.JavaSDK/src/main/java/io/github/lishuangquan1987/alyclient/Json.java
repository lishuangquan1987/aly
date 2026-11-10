package io.github.lishuangquan1987.alyclient;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.function.Function;

/** 极简 JSON 解析与取值工具（零依赖，兼容 Java 8）。 */
public final class Json {
    private Json() {
    }

    public static Object parse(String text) {
        Parser parser = new Parser(text);
        Object value = parser.parseValue();
        parser.skipWhitespace();
        if (!parser.atEnd()) {
            throw new IllegalArgumentException("trailing JSON content at " + parser.position());
        }
        return value;
    }

    @SuppressWarnings("unchecked")
    public static Map<String, Object> asObject(Object value) {
        if (value instanceof Map) {
            return (Map<String, Object>) value;
        }
        throw new IllegalArgumentException("expected JSON object, got " + describe(value));
    }

    @SuppressWarnings("unchecked")
    private static List<Object> asList(Object value) {
        if (value instanceof List) {
            return (List<Object>) value;
        }
        throw new IllegalArgumentException("expected JSON array, got " + describe(value));
    }

    public static Map<String, Object> map(Map<String, Object> data, String key) {
        Object value = data.get(key);
        return value instanceof Map ? asObject(value) : null;
    }

    public static String optStr(Map<String, Object> data, String key) {
        Object value = data.get(key);
        return value == null ? null : String.valueOf(value);
    }

    public static String str(Map<String, Object> data, String key) {
        String value = optStr(data, key);
        return value == null ? "" : value;
    }

    public static boolean bool(Map<String, Object> data, String key, boolean fallback) {
        Object value = data.get(key);
        return value instanceof Boolean ? (Boolean) value : fallback;
    }

    public static Boolean optBool(Map<String, Object> data, String key) {
        Object value = data.get(key);
        return value instanceof Boolean ? (Boolean) value : null;
    }

    public static long lng(Map<String, Object> data, String key, long fallback) {
        Object value = data.get(key);
        return value instanceof Number ? ((Number) value).longValue() : fallback;
    }

    public static Long optLng(Map<String, Object> data, String key) {
        Object value = data.get(key);
        return value instanceof Number ? ((Number) value).longValue() : null;
    }

    public static double dbl(Map<String, Object> data, String key, double fallback) {
        Object value = data.get(key);
        return value instanceof Number ? ((Number) value).doubleValue() : fallback;
    }

    public static Double optDbl(Map<String, Object> data, String key) {
        Object value = data.get(key);
        return value instanceof Number ? ((Number) value).doubleValue() : null;
    }

    public static List<String> strList(Map<String, Object> data, String key) {
        List<String> out = new ArrayList<String>();
        Object value = data.get(key);
        if (!(value instanceof List)) {
            return out;
        }
        for (Object item : asList(value)) {
            out.add(item == null ? null : String.valueOf(item));
        }
        return out;
    }

    public static List<String> optStrList(Map<String, Object> data, String key) {
        Object value = data.get(key);
        return value instanceof List ? strList(data, key) : null;
    }

    public static List<Boolean> boolList(Map<String, Object> data, String key) {
        List<Boolean> out = new ArrayList<Boolean>();
        Object value = data.get(key);
        if (!(value instanceof List)) {
            return out;
        }
        for (Object item : asList(value)) {
            out.add(item instanceof Boolean ? (Boolean) item : Boolean.FALSE);
        }
        return out;
    }

    public static List<Long> lngList(Map<String, Object> data, String key) {
        List<Long> out = new ArrayList<Long>();
        Object value = data.get(key);
        if (!(value instanceof List)) {
            return out;
        }
        for (Object item : asList(value)) {
            out.add(item instanceof Number ? ((Number) item).longValue() : null);
        }
        return out;
    }

    public static List<Double> dblList(Map<String, Object> data, String key) {
        List<Double> out = new ArrayList<Double>();
        Object value = data.get(key);
        if (!(value instanceof List)) {
            return out;
        }
        for (Object item : asList(value)) {
            out.add(item instanceof Number ? ((Number) item).doubleValue() : null);
        }
        return out;
    }

    public static <T> List<T> mapList(
            Map<String, Object> data, String key, Function<Map<String, Object>, T> mapper) {
        List<T> out = new ArrayList<T>();
        Object value = data.get(key);
        if (!(value instanceof List)) {
            return out;
        }
        for (Object item : asList(value)) {
            out.add(mapper.apply(asObject(item)));
        }
        return out;
    }

    public static <T> List<T> optMapList(
            Map<String, Object> data, String key, Function<Map<String, Object>, T> mapper) {
        Object value = data.get(key);
        if (!(value instanceof List)) {
            return null;
        }
        return mapList(data, key, mapper);
    }

    private static String describe(Object value) {
        return value == null ? "null" : value.getClass().getSimpleName();
    }

    private static final class Parser {
        private final String text;
        private int pos;

        Parser(String text) {
            this.text = text == null ? "" : text;
        }

        int position() {
            return pos;
        }

        boolean atEnd() {
            return pos >= text.length();
        }

        void skipWhitespace() {
            while (pos < text.length()) {
                char c = text.charAt(pos);
                if (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
                    pos++;
                } else {
                    break;
                }
            }
        }

        Object parseValue() {
            skipWhitespace();
            if (atEnd()) {
                throw error("unexpected end of JSON");
            }
            char c = text.charAt(pos);
            switch (c) {
                case '{':
                    return parseObject();
                case '[':
                    return parseArray();
                case '"':
                    return parseString();
                case 't':
                    expect("true");
                    return Boolean.TRUE;
                case 'f':
                    expect("false");
                    return Boolean.FALSE;
                case 'n':
                    expect("null");
                    return null;
                default:
                    if (c == '-' || (c >= '0' && c <= '9')) {
                        return parseNumber();
                    }
                    throw error("unexpected character '" + c + "'");
            }
        }

        private Map<String, Object> parseObject() {
            Map<String, Object> out = new LinkedHashMap<String, Object>();
            pos++; // {
            skipWhitespace();
            if (!atEnd() && text.charAt(pos) == '}') {
                pos++;
                return out;
            }
            while (true) {
                skipWhitespace();
                if (atEnd() || text.charAt(pos) != '"') {
                    throw error("expected object key");
                }
                String key = parseString();
                skipWhitespace();
                if (atEnd() || text.charAt(pos) != ':') {
                    throw error("expected ':' after key");
                }
                pos++;
                out.put(key, parseValue());
                skipWhitespace();
                if (atEnd()) {
                    throw error("unterminated object");
                }
                char next = text.charAt(pos++);
                if (next == '}') {
                    return out;
                }
                if (next != ',') {
                    throw error("expected ',' or '}'");
                }
            }
        }

        private List<Object> parseArray() {
            List<Object> out = new ArrayList<Object>();
            pos++; // [
            skipWhitespace();
            if (!atEnd() && text.charAt(pos) == ']') {
                pos++;
                return out;
            }
            while (true) {
                out.add(parseValue());
                skipWhitespace();
                if (atEnd()) {
                    throw error("unterminated array");
                }
                char next = text.charAt(pos++);
                if (next == ']') {
                    return out;
                }
                if (next != ',') {
                    throw error("expected ',' or ']'");
                }
            }
        }

        private String parseString() {
            pos++; // opening quote
            StringBuilder out = new StringBuilder();
            while (pos < text.length()) {
                char c = text.charAt(pos++);
                if (c == '"') {
                    return out.toString();
                }
                if (c == '\\') {
                    if (atEnd()) {
                        throw error("unterminated escape");
                    }
                    char esc = text.charAt(pos++);
                    switch (esc) {
                        case '"':
                            out.append('"');
                            break;
                        case '\\':
                            out.append('\\');
                            break;
                        case '/':
                            out.append('/');
                            break;
                        case 'b':
                            out.append('\b');
                            break;
                        case 'f':
                            out.append('\f');
                            break;
                        case 'n':
                            out.append('\n');
                            break;
                        case 'r':
                            out.append('\r');
                            break;
                        case 't':
                            out.append('\t');
                            break;
                        case 'u':
                            if (pos + 4 > text.length()) {
                                throw error("bad unicode escape");
                            }
                            String hex = text.substring(pos, pos + 4);
                            pos += 4;
                            out.append((char) Integer.parseInt(hex, 16));
                            break;
                        default:
                            throw error("bad escape '\\" + esc + "'");
                    }
                } else {
                    out.append(c);
                }
            }
            throw error("unterminated string");
        }

        private Object parseNumber() {
            int start = pos;
            if (!atEnd() && text.charAt(pos) == '-') {
                pos++;
            }
            while (!atEnd()) {
                char c = text.charAt(pos);
                if ((c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-') {
                    pos++;
                } else {
                    break;
                }
            }
            String raw = text.substring(start, pos);
            try {
                if (raw.indexOf('.') < 0 && raw.indexOf('e') < 0 && raw.indexOf('E') < 0) {
                    return Long.valueOf(raw);
                }
                return Double.valueOf(raw);
            } catch (NumberFormatException ex) {
                throw error("bad number '" + raw + "'");
            }
        }

        private void expect(String literal) {
            if (!text.startsWith(literal, pos)) {
                throw error("expected '" + literal + "'");
            }
            pos += literal.length();
        }

        private IllegalArgumentException error(String message) {
            return new IllegalArgumentException(message + " at " + pos);
        }
    }
}