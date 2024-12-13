package gwdtUtils

import (
	"crypto/md5"
	"encoding/hex"
)

//func ToString(v interface{}) string {
//	switch v := v.(type) {
//	case string:
//		return v
//	case int:
//		return strconv.Itoa(v)
//	case int64:
//		return strconv.FormatInt(v, 10)
//	case float64:
//		return strconv.FormatFloat(v, 'f', -1, 64)
//	case bool:
//		if v {
//			return "true"
//		} else {
//			return "false"
//		}
//	case []byte:
//		return string(v)
//	default:
//		return ""
//	}
//}

func MD5(s string) string {
	hash := md5.New()
	hash.Write([]byte(s))
	return hex.EncodeToString(hash.Sum(nil))
}
