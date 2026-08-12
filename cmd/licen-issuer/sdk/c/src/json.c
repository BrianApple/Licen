/* json.c - 极简 JSON 字段提取实现 */
#include "json.h"

#include <ctype.h>
#include <string.h>

/* 跳过空白 */
static const char *skip_ws(const char *p) {
    while (*p && isspace((unsigned char)*p)) {
        p++;
    }
    return p;
}

/* 查找 "field" 后接冒号的位置（跨空白），找不到返回 NULL */
static const char *find_field(const char *json, const char *field) {
    size_t flen = strlen(field);
    const char *p = json;
    while ((p = strstr(p, field)) != NULL) {
        /* 必须是完整字段名 "field"：前后都是引号 */
        if ((p == json || p[-1] == '"') && p[flen] == '"') {
            const char *q = skip_ws(p + flen + 1);
            if (*q == ':') {
                return skip_ws(q + 1);
            }
        }
        p += flen;
    }
    return NULL;
}

int licen_json_get_string(const char *json, const char *field, char *out, size_t out_len) {
    const char *v = find_field(json, field);
    size_t i = 0;
    if (v == NULL || *v != '"' || out_len == 0) {
        return -1;
    }
    v++;
    while (*v && *v != '"' && i + 1 < out_len) {
        if (*v == '\\' && v[1]) {
            v++;
            switch (*v) {
                case 'n': out[i++] = '\n'; break;
                case 't': out[i++] = '\t'; break;
                case 'r': out[i++] = '\r'; break;
                default: out[i++] = *v; break;
            }
        } else {
            out[i++] = *v;
        }
        v++;
    }
    out[i] = '\0';
    return 0;
}

int licen_json_get_int(const char *json, const char *field, int def) {
    const char *v = find_field(json, field);
    int neg = 0, val = 0;
    if (v == NULL) {
        return def;
    }
    v = skip_ws(v);
    if (*v == '-') {
        neg = 1;
        v++;
    }
    while (*v >= '0' && *v <= '9') {
        val = val * 10 + (*v - '0');
        v++;
    }
    return neg ? -val : val;
}

int licen_json_get_bool(const char *json, const char *field, int def) {
    const char *v = find_field(json, field);
    if (v == NULL) {
        return def;
    }
    if (strncmp(v, "true", 4) == 0) {
        return 1;
    }
    if (strncmp(v, "false", 5) == 0) {
        return 0;
    }
    return def;
}

int licen_json_get_array(const char *json, const char *field, char *out, size_t out_len) {
    const char *v = find_field(json, field);
    size_t i = 0;
    if (v == NULL) {
        return -1;
    }
    v = skip_ws(v);
    if (*v != '[') {
        return -1;
    }
    v++;
    out[0] = '\0';
    while (*v && *v != ']' && i + 1 < out_len) {
        v = skip_ws(v);
        if (*v == '"') {
            v++;
            while (*v && *v != '"' && i + 1 < out_len) {
                if (*v == '\\' && v[1]) {
                    v++;
                }
                out[i++] = *v++;
            }
            if (*v == '"') {
                v++;
            }
            out[i++] = ',';
        } else {
            v++;
        }
    }
    /* 去掉尾部逗号 */
    while (i > 0 && (out[i - 1] == ',' || out[i - 1] == ' ')) {
        i--;
    }
    out[i] = '\0';
    return 0;
}
