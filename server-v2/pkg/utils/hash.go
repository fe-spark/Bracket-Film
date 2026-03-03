package utils

import (
	"fmt"
	"hash/fnv"
	"regexp"
)

// GenerateHashKey 存储播放源信息时对影片名称进行处理, 提高各站点间同一影片的匹配度
func GenerateHashKey[K string | ~int | int64](key K) string {
	mName := fmt.Sprint(key)
	// 1. 去除name中的所有空格
	mName = regexp.MustCompile(`\s`).ReplaceAllString(mName, "")
	// 2. 去除name中含有的别名～.*～
	mName = regexp.MustCompile(`～.*～$`).ReplaceAllString(mName, "")
	// 3. 去除name首尾的标点符号
	mName = regexp.MustCompile(`^[[:punct:]]+|[[:punct:]]+$`).ReplaceAllString(mName, "")
	// 部分站点包含 动画版, 特殊别名 等字符, 需进行删除
	mName = regexp.MustCompile(`季.*`).ReplaceAllString(mName, "季")
	// 4. 将处理完成后的name转化为hash值作为存储时的key
	h := fnv.New32a()
	_, err := h.Write([]byte(mName))
	if err != nil {
		return ""
	}
	return fmt.Sprint(h.Sum32())
}
