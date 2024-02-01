package curl_response

import (
	"fmt"
	"strconv"
)

func Decimal(num float64) float64 {
	num, _ = strconv.ParseFloat(fmt.Sprintf("%.4f", num), 64)
	return num
}
