/* json.h - 极简 JSON 字段提取（仅支持扁平的字符串/数字/布尔/字符串数组） */
#ifndef LICEN_JSON_H
#define LICEN_JSON_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/* 提取字符串字段 "field": "value"（含转义处理）。返回 0 成功。 */
int licen_json_get_string(const char *json, const char *field, char *out, size_t out_len);

/* 提取整数字段。找不到返回 def。 */
int licen_json_get_int(const char *json, const char *field, int def);

/* 提取布尔字段。找不到返回 def。 */
int licen_json_get_bool(const char *json, const char *field, int def);

/* 提取字符串数组 "features": ["a","b"] 拼接为 "a,b"。返回 0 成功（含空数组）。 */
int licen_json_get_array(const char *json, const char *field, char *out, size_t out_len);

#ifdef __cplusplus
}
#endif

#endif /* LICEN_JSON_H */
