package utils

import "bytes"


func GenCompareCondition(buff *bytes.Buffer, prefix string, isDesc bool, leftLiteral string, rightLiteral string) {
	buff.WriteRune(' ')
	buff.WriteString(prefix)
	buff.WriteRune(' ')
	buff.WriteString(leftLiteral)
	if isDesc {
		buff.WriteRune('<')
	} else {
		buff.WriteRune('>')
	}
	buff.WriteString(rightLiteral)
}
