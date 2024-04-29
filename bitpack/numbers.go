package bitpack

import (
	"unsafe"
)

const (
	stringEncUin64DictLen   = 62
	stringEncUint64MaxBytes = 11
)

var (
	encodeLookup = [62]byte{
		0: 48, 1: 49, 2: 50, 3: 51, 4: 52, 5: 53, 6: 54, 7: 55, 8: 56, 9: 57, 36: 65, 37: 66, 38: 67,
		39: 68, 40: 69, 41: 70, 42: 71, 43: 72, 44: 73, 45: 74, 46: 75, 47: 76, 48: 77, 49: 78, 50: 79,
		51: 80, 52: 81, 53: 82, 54: 83, 55: 84, 56: 85, 57: 86, 58: 87, 59: 88, 60: 89, 61: 90, 10: 97,
		11: 98, 12: 99, 13: 100, 14: 101, 15: 102, 16: 103, 17: 104, 18: 105, 19: 106, 20: 107, 21: 108,
		22: 109, 23: 110, 24: 111, 25: 112, 26: 113, 27: 114, 28: 115, 29: 116, 30: 117, 31: 118, 32: 119,
		33: 120, 34: 121, 35: 122,
	}

	decodeLookup = [123]uint64{
		48: 0, 49: 1, 50: 2, 51: 3, 52: 4, 53: 5, 54: 6, 55: 7, 56: 8, 57: 9, 65: 36, 66: 37, 67: 38, 68: 39,
		69: 40, 70: 41, 71: 42, 72: 43, 73: 44, 74: 45, 75: 46, 76: 47, 77: 48, 78: 49, 79: 50, 80: 51, 81: 52,
		82: 53, 83: 54, 84: 55, 85: 56, 86: 57, 87: 58, 88: 59, 89: 60, 90: 61, 97: 10, 98: 11, 99: 12, 100: 13,
		101: 14, 102: 15, 103: 16, 104: 17, 105: 18, 106: 19, 107: 20, 108: 21, 109: 22, 110: 23, 111: 24,
		112: 25, 113: 26, 114: 27, 115: 28, 116: 29, 117: 30, 118: 31, 119: 32, 120: 33, 121: 34, 122: 35,
	}
)

// EncodeUint64ToString converts a uint64 to the smallest possible strinng representation using
// only alphanumeric characters (compatible e.g. with filesystem limitations)
func EncodeUint64ToString(num uint64) string {
	return EncodeUint64ToStringBuf(num, nil)
}

// EncodeUint64ToStringBuf converts a uint64 to the smallest possible strinng representation using
// only alphanumeric characters (compatible e.g. with filesystem limitations) using a buffer (must
// have sufficient size
func EncodeUint64ToStringBuf(num uint64, buf []byte) string {

	// Trivial case
	if num == 0 {
		return "0"
	}

	// If no buffer was provided, allocate just enough space
	if buf == nil {
		buf = make([]byte, stringEncUint64MaxBytes)
	}

	// Encode the number into the buffer
	n := EncodeUint64ToByteBuf(num, buf)

	// Subslice to string length and cast to string (zero-allocation)
	buf = buf[0:n]
	return *(*string)(unsafe.Pointer(&buf)) // #nosec G103
}

// EncodeUint64ToByteBuf converts a uint64 to the smallest possible byte representation using
// only alphanumeric characters (compatible e.g. with filesystem limitations) using a buffer (must
// have sufficient size
func EncodeUint64ToByteBuf(num uint64, buf []byte) (n int) {

	// Trivial case
	if num == 0 {
		buf[0] = 48 // "0"
		return 1
	}

	return encodeUint64ToByteBuf(num, buf)
}

func encodeUint64ToByteBuf(num uint64, buf []byte) (n int) {

	// Consecutively reduce the input and append character runes to the string
	for num > 0 {
		buf[n] = encodeLookup[num%stringEncUin64DictLen]
		num /= stringEncUin64DictLen
		n++
	}
	return
}

// DecodeUint64FromString converts a string representation of a uint64 back to its numeric representation
func DecodeUint64FromString(enc string) (res uint64) {
	for i := len(enc); i > 0; i-- {
		res *= stringEncUin64DictLen
		res += decodeLookup[enc[i-1]]
	}
	return
}
