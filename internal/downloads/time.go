package downloads

import "time"

func unixMilli() int64 { return time.Now().UnixMilli() }
